// Pacote erros define o erro de domínio com os códigos da tabela de contracts/api-http.md.
// Os serviços devolvem *Erro; a API traduz o código para o status HTTP.
package erros

import (
	"errors"
	"fmt"
)

// Códigos de erro do contrato.
const (
	NaoAutorizado     = "nao_autorizado"
	HostInvalido      = "host_invalido"
	NaoEncontrado     = "nao_encontrado"
	Validacao         = "validacao"
	Conflito          = "conflito"
	TransicaoInvalida = "transicao_invalida"
	VariaveisFaltando = "variaveis_faltando"
	ContaIndisponivel = "conta_indisponivel"
	SemWhatsApp       = "sem_whatsapp"
	ForaDoPrazo       = "fora_do_prazo"
	AnexoGrandeDemais = "anexo_grande_demais"
	TipoNaoSuportado  = "tipo_nao_suportado"
	WhatsAppErro      = "whatsapp_erro"
	Interno           = "interno"

	// Feature 002 — automações (specs/002-automacoes/contracts/api-http.md).
	DefinicaoInvalida  = "definicao_invalida"
	CompilacaoFalhou   = "compilacao_falhou"
	RunnerIndisponivel = "runner_indisponivel"
	IANaoConfigurada   = "ia_nao_configurada"
	IAErro             = "ia_erro"
)

// Status HTTP de cada código.
var Status = map[string]int{
	NaoAutorizado:     401,
	HostInvalido:      403,
	NaoEncontrado:     404,
	Validacao:         422,
	Conflito:          409,
	TransicaoInvalida: 409,
	VariaveisFaltando: 422,
	ContaIndisponivel: 409,
	SemWhatsApp:       422,
	ForaDoPrazo:       409,
	AnexoGrandeDemais: 413,
	TipoNaoSuportado:  415,
	WhatsAppErro:      502,
	Interno:           500,

	DefinicaoInvalida:  422,
	CompilacaoFalhou:   422,
	RunnerIndisponivel: 503,
	IANaoConfigurada:   409,
	IAErro:             502,
}

// Erro é um erro de domínio exibível ao usuário (mensagem em pt-BR).
type Erro struct {
	Codigo   string
	Mensagem string
	Detalhes map[string]any
}

func (e *Erro) Error() string { return e.Codigo + ": " + e.Mensagem }

// Novo cria um erro.
func Novo(codigo, mensagem string) *Erro { return &Erro{Codigo: codigo, Mensagem: mensagem} }

// ComDetalhes cria um erro com detalhes.
func ComDetalhes(codigo, mensagem string, detalhes map[string]any) *Erro {
	return &Erro{Codigo: codigo, Mensagem: mensagem, Detalhes: detalhes}
}

// Campos cria um erro de validação com detalhes.campos.
func Campos(campos map[string]string) *Erro {
	msg := "Confira os campos destacados."
	if len(campos) == 1 {
		for _, m := range campos {
			msg = m
		}
	}
	return &Erro{Codigo: Validacao, Mensagem: msg, Detalhes: map[string]any{"campos": campos}}
}

// Campo cria um erro de validação para um único campo.
func Campo(campo, mensagem string) *Erro { return Campos(map[string]string{campo: mensagem}) }

// NaoAchado cria um erro nao_encontrado para o recurso.
func NaoAchado(recurso string) *Erro {
	return Novo(NaoEncontrado, fmt.Sprintf("%s não encontrado.", recurso))
}

// NaoAchada é a variante no feminino (conta, conversa, mensagem, etiqueta).
func NaoAchada(recurso string) *Erro {
	return Novo(NaoEncontrado, fmt.Sprintf("%s não encontrada.", recurso))
}

// Transicao cria um erro transicao_invalida com o estado atual.
func Transicao(mensagem, estadoAtual string) *Erro {
	return ComDetalhes(TransicaoInvalida, mensagem, map[string]any{"estado_atual": estadoAtual})
}

// Como extrai *Erro de err.
func Como(err error) (*Erro, bool) {
	var e *Erro
	ok := errors.As(err, &e)
	return e, ok
}

// Eh indica se err é um *Erro com o código.
func Eh(err error, codigo string) bool {
	e, ok := Como(err)
	return ok && e.Codigo == codigo
}
