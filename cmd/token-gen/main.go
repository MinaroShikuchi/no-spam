package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	secretEnv := os.Getenv("JWT_SECRET")
	if secretEnv == "" {
		secretEnv = "super-secret-key-change-me"
	}

	secret := flag.String("secret", secretEnv, "JWT Secret Key")
	issuer := flag.String("issuer", "no-spam-admin", "Token Issuer")
	role := flag.String("role", "subscriber", "Role: 'publisher' or 'subscriber'")
	flag.Parse()

	if *role != "publisher" && *role != "subscriber" {
		log.Fatalf("Invalid role: %s. Must be 'publisher' or 'subscriber'", *role)
	}

	// Create the Claims
	claims := jwt.MapClaims{
		"iss":  *issuer,
		"role": *role,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour * 24 * 365).Unix(), // 1 year
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(*secret))
	if err != nil {
		log.Fatalf("Error signing token: %v", err)
	}

	fmt.Println(signedToken)
}
