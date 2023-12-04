package app

func (s *Server) setRoutes() {
	s.router.Use(s.Middleware())
	s.router.HandleFunc("/v3", s.handleV3)
	s.router.HandleFunc("/v3{sub?:\\@(.*)}", s.handleV3)

	s.router.HandleFunc("/health", s.HandleHealth)
}
