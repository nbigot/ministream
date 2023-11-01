package web

import "github.com/gofiber/fiber/v2"

func (w *WebAPIServer) GetStreamPropertiesForABAC(c *fiber.Ctx) (interface{}, error) {
	streamUuid, stream, err := w.GetStreamFromParameter(c)
	if err != nil {
		return nil, err
	}
	properties := map[string]interface{}{
		"streamUUID": streamUuid.String(),
		"properties": stream.GetInfo().Properties,
	}
	return properties, nil
}
