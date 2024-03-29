package web

import (
	"crypto/rand"
	"errors"
	"time"

	"github.com/nbigot/ministream/auth"
	"github.com/nbigot/ministream/config"
	"github.com/nbigot/ministream/constants"
	"github.com/nbigot/ministream/rbac"
	"github.com/nbigot/ministream/web/apierror"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type JWTManager struct {
	config config.JWTConfig
}

var JWTMgr JWTManager

func JWTProtected() func(*fiber.Ctx) error {
	return jwtware.New(jwtware.Config{
		Filter: func(c *fiber.Ctx) bool {
			return !JWTMgr.config.Enable
		},
		SigningKey:     []byte(JWTMgr.config.SecretKey),
		SuccessHandler: JWTPostValidate,
		ErrorHandler:   jwtError,
		ContextKey:     constants.JWTContextKey,
		SigningMethod:  "HS256",
	})
}

func jwtError(c *fiber.Ctx, err error) error {
	if err.Error() == "Missing or malformed JWT" {
		httpError := apierror.APIError{
			Message:  "missing or malformed jwt",
			Code:     constants.ErrorJWTMissingOrMalformed,
			HttpCode: fiber.StatusBadRequest,
		}
		return httpError.HTTPResponse(c)
	} else {
		httpError := apierror.APIError{
			Message:  "invalid or expired jwt",
			Code:     constants.ErrorJWTInvalidOrExpired,
			HttpCode: fiber.StatusUnauthorized,
		}
		return httpError.HTTPResponse(c)
	}
}

func GetJWTClaim(c *fiber.Ctx, key string) (interface{}, error) {
	value := c.Locals(constants.JWTContextKey)
	if value == nil {
		return "", errors.New("JWT not found")
	}

	token := value.(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	return claims[key], nil
}

func JWTPostValidate(c *fiber.Ctx) error {
	value := c.Locals(constants.JWTContextKey)
	if value == nil {
		httpError := apierror.APIError{
			Message:  "invalid jwt",
			Code:     constants.ErrorJWTInvalidOrExpired,
			HttpCode: fiber.StatusUnauthorized,
		}
		return httpError.HTTPResponse(c)
	}

	token := value.(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	valid := claims["iss"] == JWTMgr.config.ISS &&
		claims["sub"] == JWTMgr.config.Sub &&
		claims["aud"] == JWTMgr.config.Aud &&
		claims[constants.JWTClaimsAccountKey] == JWTMgr.config.AccountId

	if !valid {
		httpError := apierror.APIError{
			Message:  "invalid jwt",
			Code:     constants.ErrorJWTInvalidOrExpired,
			HttpCode: fiber.StatusUnauthorized,
		}
		return httpError.HTTPResponse(c)
	}

	if int64(claims["iat"].(float64)) < JWTMgr.config.RevokeEmittedBeforeDate.Unix() {
		httpError := apierror.APIError{
			Message:  "jwt was revoked",
			Code:     constants.ErrorJWTInvalidOrExpired,
			HttpCode: fiber.StatusUnauthorized,
		}
		return httpError.HTTPResponse(c)
	}

	if claims["superuser"] == true {
		c.Locals(constants.SuperUserContextKey, true)
		return c.Next()
	}

	if rbac.RbacMgr.IsEnabled() {
		aInterface := claims[constants.JWTClaimsRolesKey].([]interface{})
		roleNames := make([]string, len(aInterface))
		for i, v := range aInterface {
			roleNames[i] = v.(string)
		}
		if roles, err := rbac.RbacMgr.Rbac.GetRoles(&roleNames); err == nil {
			c.Locals(constants.UserContextKey, claims[constants.JWTClaimsUserKey])
			c.Locals(constants.RolesContextKey, roles)
		} else {
			httpError := apierror.APIError{
				Message:  "invalid jwt",
				Details:  err.Error(),
				Code:     constants.ErrorJWTRBACUnknownRole,
				HttpCode: fiber.StatusForbidden,
				Err:      err,
			}
			return httpError.HTTPResponse(c)
		}
	}

	return c.Next()
}

func (m *JWTManager) GenerateJWT(isSuperUser bool, accessKeyId string, secretAccessKey string) (bool, string, *jwt.MapClaims, *rbac.User, error) {
	// Generate a Json Web Token
	token := jwt.New(jwt.SigningMethodHS256)

	// https://datatracker.ietf.org/doc/html/rfc7519
	claims := token.Claims.(jwt.MapClaims)
	claims["iss"] = m.config.ISS                                                                      // issuer
	claims["sub"] = m.config.Sub                                                                      // subject
	claims["aud"] = m.config.Aud                                                                      // audience
	claims["exp"] = time.Now().Add(time.Minute * time.Duration(m.config.TokenExpireInMinutes)).Unix() // expiration time
	claims["iat"] = time.Now().Unix()                                                                 // issued at
	claims["jti"] = uuid.NewString()                                                                  // JWT unique ID
	claims[constants.JWTClaimsAccountKey] = m.config.AccountId                                        // account id (any customer value)

	success := false
	var err error
	var user *rbac.User

	if isSuperUser {
		claims["superuser"] = true
		success = true
	}

	// find user
	if accessKeyId != "" && secretAccessKey != "" {
		if user, err = rbac.RbacMgr.Rbac.GetUser(accessKeyId); err == nil {
			claims[constants.JWTClaimsUserKey] = user.Id
			claims[constants.JWTClaimsRolesKey] = user.GetRoles()

			// auth user
			success, err = auth.AuthMgr.AuthenticateUser(accessKeyId, secretAccessKey)
			if err != nil {
				return success, "", &claims, user, err
			}
		}
	}

	if !success {
		return success, "", &claims, user, err
	}

	t, err := token.SignedString([]byte(m.config.SecretKey))
	if err != nil {
		return success, "", &claims, user, err
	}

	return true, t, &claims, user, err
}

func (m *JWTManager) RevokeAll() {
	m.config.RevokeEmittedBeforeDate = time.Now()
}

func (m *JWTManager) Initialize(conf config.JWTConfig) {
	m.config = conf

	// generate a secret key if the default one from the configuration file is empty
	if m.config.SecretKey != "" {
		return
	}

	// note: a new key generated at program startup implies that
	// all jwt previously generated will be obseleted,
	// therefore all application/user will have to authenticate again.

	// generate a random secret key
	key := make([]byte, 256)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	m.config.SecretKey = string(key[:])
}

func (m *JWTManager) ResetConfig() {
	m.config = config.JWTConfig{}
}

func (m *JWTManager) Finalize() {
	m.ResetConfig()
}
