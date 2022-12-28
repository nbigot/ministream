package main

import (
	"flag"
	"fmt"

	"github.com/nbigot/ministream/auth"
)

func main() {
	// example 1: $ go run cmd/generatepasswords/generatepasswords.go -digest sha512 -iterations 1000 -salt randomStringAbcdef012346789 -password thisIsMySecretPassword
	// example 2: $ go run cmd/generatepasswords/generatepasswords.go -digest sha512 -iterations 1000 -count 10
	digest := flag.String("digest", "sha512", "digest type (allowed value is: sha512, sha256, md5")
	iterations := flag.Int("iterations", 1000, "number of iterations")
	pSalt := flag.String("salt", "", "a random string")
	pPassword := flag.String("password", "", "a random password")
	count := flag.Int("count", 1, "number of passwords to generate")
	flag.Parse()

	if *digest != "sha512" && *digest != "sha256" && *digest != "md5" {
		panic("digest allowed value is: sha512, sha256, md5")
	}

	if *iterations <= 0 {
		panic("iterations must be a positive integer")
	}

	salt := *pSalt
	randomSalt := false
	if *pSalt == "" {
		randomSalt = true
	}

	password := *pPassword
	randomPassword := false
	if *pPassword == "" {
		randomPassword = true
	}

	if *count <= 0 {
		panic("count must be a positive integer")
	}

	fmt.Printf("Generate %d passwords\n", *count)

	for i := 0; i < *count; i++ {
		if randomSalt {
			salt = auth.GenerateRandomSalt(20)
		}

		if randomPassword {
			password = auth.GenerateRandomPassword(32)
		}

		if hash, err := auth.HashPassword(*digest, *iterations, salt, password); err == nil {
			fmt.Printf("Digest: %s Iterations: %d Salt: %s Password: %s Hash: %s\n", *digest, *iterations, salt, password, hash)
		} else {
			panic(err)
		}
	}
}
