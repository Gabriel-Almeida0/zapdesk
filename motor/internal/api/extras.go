package api

// registrarRotasExtras registra as rotas das histórias seguintes.
func (s *Servidor) registrarRotasExtras() {
	if s.o.Servicos.Leads != nil {
		s.registrarRotasLeads()
	}
	if s.o.Servicos.Disparos != nil {
		s.registrarRotasDisparos()
	}
	if s.o.Servicos.Organizacao != nil {
		s.registrarRotasOrganizacao()
	}
	if s.o.Servicos.Automacoes != nil {
		s.registrarRotasAutomacoesComum()
		s.registrarRotasAutomacoes()
	}
	if s.o.Servicos.Funil != nil {
		s.registrarRotasFunis()
	}
	for _, f := range s.o.Servicos.Extras {
		f(s)
	}
}
