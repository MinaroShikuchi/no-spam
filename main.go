package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"net"
	"net/http"
	"no-spam/connectors"
	"no-spam/handlers"
	"no-spam/hub"
	"no-spam/middleware"
	"no-spam/store"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	certFile := flag.String("cert", "certs/cert.pem", "Path to TLS certificate file")
	keyFile := flag.String("key", "certs/key.pem", "Path to TLS key file")
	addr := flag.String("addr", ":8443", "Address to listen on")
	fcmCreds := flag.String("fcm-creds", "", "Path to Firebase credentials file (optional)")
	httpMode := flag.Bool("http", false, "Run in HTTP mode (disable TLS)")
	// fcmProjectID removed, inferred from creds
	flag.Parse()

	// Initialize Store
	s, err := store.NewSQLiteStore("no-spam.db")
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Check for admin user
	hasAdmin, err := s.HasAdminUser()
	if err != nil {
		log.Printf("[AUTH] Failed to check for admin user: %v", err)
	} else if !hasAdmin {
		// Checks if user "admin" already exists (but implies role != admin)
		user, err := s.GetUser("admin")
		if err != nil {
			log.Printf("[AUTH] Failed to check for existing 'admin' username: %v", err)
		}

		if user != nil {
			// User "admin" exists but is not an admin role (otherwise HasAdminUser would be true)
			if err := s.UpdateUserRole("admin", "admin"); err != nil {
				log.Printf("[AUTH] Failed to promote 'admin' user: %v", err)
			} else {
				log.Printf("==================================================")
				log.Printf("[AUTH] Promoted existing user 'admin' to admin role.")
				log.Printf("==================================================")
			}
		} else {
			// Generate random password
			const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			var b [8]byte
			for i := 0; i < 8; i++ {
				b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
				time.Sleep(1 * time.Nanosecond)
			}
			password := string(b[:])

			// Hash password
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				log.Fatalf("[AUTH] Failed to hash password: %v", err)
			}

			// Create Admin
			if err := s.CreateUser("admin", string(hash), "admin"); err != nil {
				log.Fatalf("[AUTH] Failed to create admin user: %v", err)
			}

			log.Printf("==================================================")
			log.Printf("[AUTH] Admin user created:")
			log.Printf("[AUTH] Username: admin")
			log.Printf("[AUTH] Password: %s", password)
			log.Printf("==================================================")
		}
	}

	// Initialize Hub
	h := hub.NewHub(s)

	// Initialize Connectors
	mockConn := connectors.NewMockConnector()
	fcmConn := connectors.NewFCMConnector(*fcmCreds)
	apnsConn := connectors.NewAPNSConnector()
	webhookConn := connectors.NewWebhookConnector()

	// Register Connectors
	h.RegisterConnector("mock", mockConn)
	h.RegisterConnector("fcm", fcmConn)
	h.RegisterConnector("apns", apnsConn)
	h.RegisterConnector("webhook", webhookConn)

	// Start background queue processor
	ctx := context.Background()
	h.StartQueueProcessor(ctx)

	// Initialize Gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Public routes (no auth)
	router.POST("/register", handlers.RegisterHandler(s))
	router.POST("/login", handlers.LoginHandler(s))

	// Authenticated routes
	auth := router.Group("/")
	auth.Use(middleware.JWTAuthMiddleware())
	{
		auth.POST("/refresh", handlers.RefreshHandler())

		// Subscriber routes
		subscribers := auth.Group("/")
		subscribers.Use(middleware.RequireRole("subscriber"))
		{
			subscribers.POST("/subscribe", handlers.SubscribeHandler(h))
			subscribers.POST("/unsubscribe", handlers.UnsubscribeHandler(h))
			subscribers.GET("/topics", handlers.TopicsHandler(h))
		}

		// Publisher routes
		publishers := auth.Group("/")
		publishers.Use(middleware.RequireRole("publisher"))
		{
			publishers.POST("/send", handlers.SendHandler(h))
			publishers.GET("/stats", handlers.StatsHandler(h))
		}

		// Admin routes
		admin := auth.Group("/admin")
		admin.Use(middleware.RequireRole("admin"))
		{
			admin.GET("/topics", handlers.ListTopicsHandler(h))
			admin.POST("/topics", handlers.CreateTopicHandler(h))
			admin.DELETE("/topics/:name", handlers.DeleteTopicHandler(h))
			admin.GET("/topics/:name/messages", handlers.GetMessagesHandler(h))
			admin.DELETE("/topics/:name/messages", handlers.ClearMessagesHandler(h))
			admin.GET("/topics/:name/subscribers", handlers.GetSubscribersHandler(h))
			admin.DELETE("/topics/:name/subscribers", handlers.ClearSubscribersHandler(h))
			admin.GET("/token", handlers.GetTokenHandler())
		}
	}

	server := &http.Server{
		Addr:    *addr,
		Handler: router,
	}

	if *httpMode {
		log.Printf("Server listening on %s (HTTP - TLS Disabled)", *addr)
		log.Printf("WARNING: Traffic is unencrypted. Ensure you are running behind a secure proxy.")
		if err := server.ListenAndServe(); err != nil {
			log.Fatal("Server failed: ", err)
		}
	} else {
		// Configure TLS 1.3 Strict
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
		}
		server.TLSConfig = tlsConfig

		log.Printf("Server listening on %s (TLS 1.3 strict)", *addr)

		// Check if cert files exist, generate if not
		if _, err := os.Stat(*certFile); os.IsNotExist(err) {
			log.Printf("Certificate file %s not found. Generating self-signed certificate...", *certFile)
			if err := generateSelfSignedCert(*certFile, *keyFile); err != nil {
				log.Fatalf("Failed to generate certificate: %v", err)
			}
			log.Printf("Successfully generated self-signed certificate at %s and %s", *certFile, *keyFile)
		} else {
			log.Printf("Found existing certificate: %s", *certFile)
		}

		if err := server.ListenAndServeTLS(*certFile, *keyFile); err != nil {
			log.Fatal("Server failed: ", err)
		}
	}
}

func generateSelfSignedCert(certPath, keyPath string) error {
	// ensure directory exists
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return err
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"no-spam"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add localhost and IP addresses
	template.DNSNames = append(template.DNSNames, "localhost")
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	// Save Cert
	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	// Save Key
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}

	return nil
}
