package web

import "github.com/gofiber/fiber/v2"

func GetStreamPropertiesForABAC(c *fiber.Ctx) (interface{}, error) {
	streamUuid, stream, err := GetStreamFromParameter(c)
	if err != nil {
		return nil, err
	}
	properties := map[string]interface{}{
		"streamUUID": streamUuid.String(),
		"properties": stream.Properties,
	}
	return properties, nil
}