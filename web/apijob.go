package web

import (
	"fmt"
	"ministream/constants"
	"ministream/stream"
	. "ministream/web/apierror"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// CreateJob godoc
// @Summary Create a job
// @Description Create a job
// @ID job-create
// @Accept json
// @Produce json
// @Tags Job
// @Success 200 {array} uuid.UUID "successful operation"
// @Router /api/v1/job/ [post]
func CreateJob(c *fiber.Ctx) error {
	job := stream.Job{} // TODO
	return c.Status(fiber.StatusCreated).JSON(job)
}

// ListJobs godoc
// @Summary List jobs
// @Description Get the list of all jobs UUIDs
// @ID job-list
// @Accept json
// @Produce json
// @Tags Job
// @Success 200 {array} uuid.UUID "successful operation"
// @Router /api/v1/jobs [get]
func ListJobs(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "success", "jobs": nil}) // TODO
}

// GetJob godoc
// @Summary Get a job
// @Description Get the job description and status
// @ID job-get
// @Accept json
// @Produce json
// @Tags Job
// @Param jobuuid path string true "Some job UUID" Format(uuid.UUID)
// @Success 200 {array} uuid.UUID "successful operation"
// @Router /api/v1/job/{jobuuid} [get]
func GetJob(c *fiber.Ctx) error {
	_, job, httpError := GetJobFromParameter(c)
	if httpError != nil {
		return httpError.HTTPResponse(c)
	}

	return c.JSON(job)
}

// DeleteJob godoc
// @Summary Delete a job
// @Description Delete a job
// @ID job-delete
// @Accept json
// @Produce json
// @Tags Job
// @Param jobuuid path string true "Some job UUID" Format(uuid.UUID)
// @Success 200 {array} uuid.UUID "successful operation"
// @Router /api/v1/job/{jobuuid} [delete]
func DeleteJob(c *fiber.Ctx) error {
	jobUuid, httpError := GetJobUUIDFromParameter(c)
	if httpError != nil {
		return httpError.HTTPResponse(c)
	}

	// TODO
	return c.JSON(jobUuid)
}

func GetJobUUIDFromParameter(c *fiber.Ctx) (stream.JobUUID, *APIError) {
	jobUuid, err := uuid.Parse(c.Params("jobuuid"))
	if err != nil {
		// param is not a valid UUID
		return jobUuid, &APIError{
			Message:  fmt.Sprintf("'%s' is not a valid stream uuid", c.Params("jobuuid")),
			Code:     constants.ErrorInvalidJobUuid,
			HttpCode: fiber.StatusBadRequest,
			Err:      err,
		}
	}

	// job uuid was not found found in the exsting jobs
	return jobUuid, nil
}

func GetJobFromParameter(c *fiber.Ctx) (stream.JobUUID, *stream.Job, *APIError) {
	jobUuid, err := GetJobUUIDFromParameter(c)
	if err != nil {
		return jobUuid, nil, err
	}

	jobPtr := stream.GetJob(jobUuid)
	if jobPtr == nil {
		// job was found
		return jobUuid, nil, &APIError{
			Message:    fmt.Sprintf("job '%s' was not found", c.Params("jobuuid")),
			Code:       constants.ErrorJobUuidNotFound,
			HttpCode:   fiber.StatusNoContent,
			StreamUUID: jobUuid,
		}
	}

	// job uuid was not found found in the exsting jobs
	return jobUuid, jobPtr, nil
}
