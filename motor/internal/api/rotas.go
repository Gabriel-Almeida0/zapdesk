package api

// registrarRotasDominio registra as rotas das histórias de usuário.
func (s *Servidor) registrarRotasDominio() {
	sv := s.o.Servicos
	if sv.Contas != nil {
		s.registrarRotasContas()
	}
	if sv.Chat != nil {
		s.registrarRotasConversas()
		s.registrarRotasMensagens()
		s.registrarRotasContatos()
		s.registrarRotasStatus()
	}
	if sv.Arquivos != nil {
		s.registrarRotasArquivos()
	}
	s.registrarRotasExtras()
}
