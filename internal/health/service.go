package health

import (
	"context"
)

type Service interface {
	Check(ctx context.Context) HealthStatus
}

type HealthStatus struct {
	Status  		string 	`json:"status"`
	StatusCode 		int 	`json:"statusCode"`
	Message 		string 	`json:"message"`
}

type service struct{}

func NewService() Service {
	return &service{}
}

func (s *service) Check(ctx context.Context) HealthStatus {
	return HealthStatus{
		Status:  		"ok",
		StatusCode: 	200,
		Message: 		"Service is healthy",
	}
}