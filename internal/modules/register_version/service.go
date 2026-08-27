package registerversion

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetRegisterVersion() (*SystemPreference, error) {
	return s.repo.GetRegisterVersion()
}
