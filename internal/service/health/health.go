package service

type HealthService struct {
	// kafka health, etc...
}

func NewHealthService() *HealthService {
	return &HealthService{}
}

func (s *HealthService) Check() string {
	return "ok"
}