package service

var StreamService *Service = nil

func NewGlobalService() {
	StreamService = NewService()
}
