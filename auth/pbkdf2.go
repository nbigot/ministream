package auth

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"ministream/constants"
	"regexp"
	"strconv"

	"golang.org/x/crypto/pbkdf2"
)

type Pbkdf2 struct {
	digestId   int    // The HMAC digest as int (stored for more performance)
	digest     string // The HMAC digest algorithm applied to derive a key of the input password.
	iterations int    // The number of iterations desired. The higher the number of iterations, the more secure the derived key will be, but will take a longer amount of time to complete.
	salt       string // A sequence of bits, known as a cryptographic salt encoded in B64.
	hash       string // The computed derived key by the pbkdf2 algorithm encoded in B64.
}

var reg *regexp.Regexp

func HashedPasswordToPbkdf2(hash string) (*Pbkdf2, error) {
	if hash == "" {
		return nil, fmt.Errorf("empty hash")
	}

	parts := reg.FindStringSubmatch(hash)
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid hash: %s", hash)
	}

	if digestId, err := DigestToAuthMethod(parts[1]); err != nil {
		return nil, err
	} else {
		iterations, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid iterations: %s", parts[2])
		}
		p := Pbkdf2{
			digestId:   digestId,
			digest:     parts[1],
			iterations: iterations,
			salt:       parts[3],
			hash:       parts[4],
		}
		return &p, nil
	}
}

func (p *Pbkdf2) Verify(password string) (bool, error) {
	if hashedPassword, err := p.Hash(password); err != nil {
		return false, err
	} else {
		if hashedPassword == p.hash {
			return true, nil
		} else {
			return false, nil
		}
	}
}

func (p *Pbkdf2) Hash(password string) (string, error) {
	switch p.digestId {
	case constants.AuthHashMethodSHA256:
		dk := pbkdf2.Key([]byte(password), []byte(p.salt), p.iterations, sha256.Size, sha256.New)
		hashedPassword := fmt.Sprintf("%x", dk)
		return hashedPassword, nil
	case constants.AuthHashMethodSHA512:
		dk := pbkdf2.Key([]byte(password), []byte(p.salt), p.iterations, sha512.Size, sha512.New)
		hashedPassword := fmt.Sprintf("%x", dk)
		return hashedPassword, nil
	case constants.AuthHashMethodSHA1:
		dk := pbkdf2.Key([]byte(password), []byte(p.salt), p.iterations, sha1.Size, sha1.New)
		hashedPassword := fmt.Sprintf("%x", dk)
		return hashedPassword, nil
	case constants.AuthHashMethodMD5:
		dk := pbkdf2.Key([]byte(password), []byte(p.salt), p.iterations, md5.Size, md5.New)
		hashedPassword := fmt.Sprintf("%x", dk)
		return hashedPassword, nil
	case constants.AuthHashMethodNone:
		return password, nil
	default:
		return "", fmt.Errorf("not implemented hash method: %s", p.digest)
	}
}

func (p *Pbkdf2) ToString() string {
	return fmt.Sprintf("$pbkdf2-%s$i=%d$%s$%s", p.digest, p.iterations, p.salt, p.hash)
}

func DigestToAuthMethod(digest string) (int, error) {
	switch digest {
	case "md5":
		return constants.AuthHashMethodMD5, nil
	case "sha256":
		return constants.AuthHashMethodSHA256, nil
	case "sha512":
		return constants.AuthHashMethodSHA512, nil
	default:
		return -1, fmt.Errorf("unknown auth digest method: %s", digest)
	}
}

func DigestToHashMethod(digest string) (int, int, func() hash.Hash, error) {
	switch digest {
	case "sha256":
		return constants.AuthHashMethodSHA256, sha256.Size, sha256.New, nil
	case "sha512":
		return constants.AuthHashMethodSHA512, sha512.Size, sha512.New, nil
	case "sha1":
		return constants.AuthHashMethodSHA1, sha1.Size, sha1.New, nil
	case "md5":
		return constants.AuthHashMethodMD5, md5.Size, md5.New, nil
	default:
		return 0, 0, nil, fmt.Errorf("not implemented hash method: %s", digest)
	}
}

func HashPassword(digest string, iterations int, salt string, password string) (string, error) {
	if digestId, keylen, h, err := DigestToHashMethod(digest); err != nil {
		return "", err
	} else {
		dk := pbkdf2.Key([]byte(password), []byte(salt), iterations, keylen, h)
		hashedPassword := fmt.Sprintf("%x", dk)
		p := Pbkdf2{
			digestId:   digestId,
			digest:     digest,
			iterations: iterations,
			salt:       salt,
			hash:       hashedPassword,
		}
		return p.ToString(), nil
	}
}

func init() {
	// string format is $pbkdf2-<digest>$i=<iterations>$<salt>$<hash>
	reg, _ = regexp.Compile("^\\$pbkdf2-([^$]+)\\$i=(\\d+)\\$([^$]+)\\$([^\r\n\t ]+)")
}
