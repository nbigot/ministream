package service

var StreamService *Service = nil

func NewGlobalService() {
	StreamService = NewService()
}

func Stop() {
	StreamService.Stop()
}
