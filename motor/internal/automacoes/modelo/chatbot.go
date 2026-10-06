package modelo

import (
	"fmt"
	"regexp"

	"zapdesk/motor/internal/variaveis"
)

// VariaveisPadrao estão sempre disponíveis nos textos (data-model › Resolução de variáveis).
var VariaveisPadrao = map[string]bool{"nome": true, "primeiro_nome": true, "telefone": true, "ultima_mensagem": true, "conta": true}

// NormalizarChatbot gera ids ausentes das ações dentro de nós e fixa versao=1.
func NormalizarChatbot(d *DefinicaoChatbot) {
	if d.Versao == 0 {
		d.Versao = 1
	}
	for i := range d.Nos {
		if d.Nos[i].Acao != nil && d.Nos[i].Acao.ID == "" {
			d.Nos[i].Acao.ID = d.Nos[i].ID
		}
	}
}

// ValidarChatbot valida o grafo (caminho base "definicao"). Regras de formatos.md: exatamente um
// início; ids únicos; referências existentes; todos alcançáveis; saídas obrigatórias; variável
// de sessão usada só depois de capturada; aguardar/iniciar_chatbot proibidos; 2–200 nós.
//
// Decisão (registrada na tarefa T019): a exigência "variável capturada antes" é aplicada às regras
// `variavel` das condições e aos `{x}` de textos cujo nome é capturado por alguma pergunta do grafo;
// `{x}` que nenhuma pergunta captura é tratado como campo do lead/variável padrão (resolvido em
// execução; sem valor → a ação falha com "Variável {x} sem valor").
func ValidarChatbot(d *DefinicaoChatbot) []ErroDefinicao {
	c := &coletor{}
	if d == nil {
		c.add("definicao", "Informe a definição do chatbot.")
		return c.erros
	}
	if d.Versao != 1 {
		c.add("definicao.versao", "Versão de definição não suportada (use 1).")
	}
	if vazio(d.NaoEntendi) || tamanho(d.NaoEntendi) > 500 {
		c.add("definicao.nao_entendi", "A mensagem de \"não entendi\" deve ter de 1 a 500 caracteres.")
	}
	if d.MaxTentativas < 1 || d.MaxTentativas > 10 {
		c.add("definicao.max_tentativas", "Use de 1 a 10 tentativas.")
	}
	if d.InatividadeMin < 1 || d.InatividadeMin > 1440 {
		c.add("definicao.inatividade_min", "A inatividade deve ficar entre 1 e 1.440 minutos.")
	}
	if len(d.Nos) < MinNos || len(d.Nos) > MaxNos {
		c.add("definicao.nos", "O chatbot deve ter de 2 a 200 nós.")
	}

	indice := map[string]int{}
	inicios := 0
	for i, n := range d.Nos {
		p := fmt.Sprintf("definicao.nos[%d]", i)
		id := n.ID
		c.noID = &id
		if !IDValido(n.ID) {
			c.add(p+".id", "Id de nó inválido (use letras, números, _ e -, até 40).")
		} else if _, dup := indice[n.ID]; dup {
			c.add(p+".id", "Id de nó repetido: %s.", n.ID)
		} else {
			indice[n.ID] = i
		}
		if n.Tipo == NoInicio {
			inicios++
		}
		c.noID = nil
	}
	if inicios != 1 {
		c.add("definicao.nos", "O chatbot deve ter exatamente um nó de início.")
	}
	if i, ok := indice[d.Inicio]; !ok || d.Nos[i].Tipo != NoInicio {
		c.add("definicao.inicio", "O início deve apontar para o nó do tipo início.")
	}

	existe := func(p, campo, destino string) {
		if destino == "" {
			c.add(p+"."+campo, "Escolha o próximo nó.")
			return
		}
		if _, ok := indice[destino]; !ok {
			c.add(p+"."+campo, "Nó %q não existe.", destino)
		}
	}
	opcional := func(p, campo string, destino *string) {
		if destino != nil && *destino != "" {
			existe(p, campo, *destino)
		}
	}
	for i, n := range d.Nos {
		p := fmt.Sprintf("definicao.nos[%d]", i)
		id := n.ID
		c.noID = &id
		switch n.Tipo {
		case NoInicio:
			existe(p, "proximo", n.Proximo)
		case NoMensagem:
			if (n.TemplateID == nil || *n.TemplateID == "") && (vazio(n.Texto) || tamanho(n.Texto) > MaxTextoMensagem) {
				c.add(p+".texto", "O texto deve ter de 1 a 4.096 caracteres.")
			}
			existe(p, "proximo", n.Proximo)
		case NoMenu:
			if vazio(n.Texto) || tamanho(n.Texto) > MaxTextoMensagem {
				c.add(p+".texto", "O texto deve ter de 1 a 4.096 caracteres.")
			}
			if len(n.Opcoes) < 1 || len(n.Opcoes) > MaxOpcoesMenu {
				c.add(p+".opcoes", "O menu deve ter de 1 a 10 opções.")
			}
			for j, o := range n.Opcoes {
				po := fmt.Sprintf("%s.opcoes[%d]", p, j)
				if vazio(o.Rotulo) || tamanho(o.Rotulo) > 100 {
					c.add(po+".rotulo", "O rótulo deve ter de 1 a 100 caracteres.")
				}
				existe(po, "proximo", o.Proximo)
			}
			opcional(p, "ao_esgotar", n.AoEsgotar)
		case NoPergunta:
			if vazio(n.Texto) || tamanho(n.Texto) > MaxTextoMensagem {
				c.add(p+".texto", "O texto deve ter de 1 a 4.096 caracteres.")
			}
			if n.Variavel == nil || !VariavelValida(*n.Variavel) {
				c.add(p+".variavel", "Nome de variável inválido (minúsculas, números e _, até 40).")
			}
			if v := n.Validacao; v != nil {
				switch v.Tipo {
				case "nenhuma", "email", "numero", "telefone":
				case "regex":
					pad := ""
					if v.Padrao != nil {
						pad = *v.Padrao
					}
					validarRegex(c, p+".validacao.padrao", pad)
				default:
					c.add(p+".validacao.tipo", "Use nenhuma, email, numero, telefone ou regex.")
				}
			}
			existe(p, "proximo", n.Proximo)
			opcional(p, "ao_esgotar", n.AoEsgotar)
		case NoCondicao:
			if len(n.Ramos) == 0 {
				c.add(p+".ramos", "Informe pelo menos um ramo.")
			}
			for j, r := range n.Ramos {
				pr := fmt.Sprintf("%s.ramos[%d]", p, j)
				cd := r.Condicoes
				validarCondicoes(c, pr+".condicoes", &cd)
				existe(pr, "proximo", r.Proximo)
			}
			existe(p, "senao", n.Senao)
		case NoAcao:
			if n.Acao == nil {
				c.add(p+".acao", "Escolha a ação.")
			} else {
				validarAcao(c, p+".acao", *n.Acao, contextoAcao{chatbot: true})
			}
			existe(p, "proximo", n.Proximo)
		case NoIA:
			if n.AutomacaoID == "" {
				c.add(p+".automacao_id", "Escolha a automação de IA.")
			}
			switch n.ModoIA {
			case "responder":
			case "variavel":
				if n.Variavel == nil || !VariavelValida(*n.Variavel) {
					c.add(p+".variavel", "Informe a variável que recebe a resposta.")
				}
			default:
				c.add(p+".modo", "Use responder ou variavel.")
			}
			existe(p, "proximo", n.Proximo)
			opcional(p, "em_erro", n.EmErro)
		case NoHumano, NoFim:
			if n.MensagemHumFim != nil && tamanho(*n.MensagemHumFim) > MaxTextoMensagem {
				c.add(p+".mensagem", "A mensagem pode ter até 4.096 caracteres.")
			}
			if n.Proximo != "" {
				c.add(p+".proximo", "Este nó encerra a conversa com o bot e não tem saída.")
			}
		case "":
			c.add(p+".tipo", "Informe o tipo do nó.")
		default:
			c.add(p+".tipo", "Tipo de nó desconhecido: %s.", n.Tipo)
		}
		c.noID = nil
	}
	if len(c.erros) > 0 {
		return c.erros
	}

	// Alcançabilidade a partir do início.
	alcancados := alcancaveis(d, d.Inicio, indice)
	for i, n := range d.Nos {
		if !alcancados[n.ID] {
			id := n.ID
			c.noID = &id
			c.add(fmt.Sprintf("definicao.nos[%d]", i), "Nó %q não é alcançável a partir do início.", n.ID)
			c.noID = nil
		}
	}

	// Variáveis capturadas antes do uso.
	capturadoPor := map[string][]string{} // variável → nós que a capturam
	for _, n := range d.Nos {
		for _, v := range variaveisCapturadas(n) {
			capturadoPor[v] = append(capturadoPor[v], n.ID)
		}
	}
	antes := func(v, alvo string) bool {
		for _, origem := range capturadoPor[v] {
			if origem == alvo {
				continue
			}
			if alcancaveis(d, origem, indice)[alvo] && origem != alvo {
				return true
			}
		}
		return false
	}
	for i, n := range d.Nos {
		p := fmt.Sprintf("definicao.nos[%d]", i)
		id := n.ID
		c.noID = &id
		for _, v := range variaveisUsadasTexto(n) {
			if VariaveisPadrao[v] || len(capturadoPor[v]) == 0 {
				continue
			}
			if !antes(v, n.ID) {
				c.add(p, "A variável {%s} é usada antes de ser perguntada.", v)
			}
		}
		for _, v := range variaveisUsadasCondicao(n) {
			if VariaveisPadrao[v] {
				continue
			}
			if !antes(v, n.ID) {
				c.add(p, "A variável %q é usada antes de ser perguntada.", v)
			}
		}
		c.noID = nil
	}
	return c.erros
}

func alcancaveis(d *DefinicaoChatbot, de string, indice map[string]int) map[string]bool {
	vistos := map[string]bool{}
	fila := []string{}
	if i, ok := indice[de]; ok {
		fila = append(fila, d.Nos[i].Saidas()...)
	}
	vistos[de] = true
	// "de" só conta como alcançado por si se houver ciclo; tratamos o início como alcançado.
	for len(fila) > 0 {
		id := fila[0]
		fila = fila[1:]
		if vistos[id] {
			continue
		}
		vistos[id] = true
		if i, ok := indice[id]; ok {
			fila = append(fila, d.Nos[i].Saidas()...)
		}
	}
	return vistos
}

func variaveisCapturadas(n No) []string {
	var l []string
	switch n.Tipo {
	case NoPergunta:
		if n.Variavel != nil {
			l = append(l, *n.Variavel)
		}
	case NoIA:
		if n.ModoIA == "variavel" && n.Variavel != nil {
			l = append(l, *n.Variavel)
		}
	case NoAcao:
		if n.Acao != nil && n.Acao.Tipo == AcaoExecutarIA && n.Acao.SalvarEm != nil {
			l = append(l, *n.Acao.SalvarEm)
		}
	}
	return l
}

func variaveisUsadasTexto(n No) []string {
	var textos []string
	textos = append(textos, n.Texto)
	if n.MensagemHumFim != nil {
		textos = append(textos, *n.MensagemHumFim)
	}
	if n.Acao != nil {
		textos = append(textos, n.Acao.Texto, n.Acao.Titulo)
		if n.Acao.Valor != nil {
			textos = append(textos, *n.Acao.Valor)
		}
		textos = append(textos, textosDeMapa(n.Acao.Entrada)...)
	}
	textos = append(textos, textosDeMapa(n.Entrada)...)
	var l []string
	for _, t := range textos {
		l = append(l, variaveis.Extrair(t)...)
	}
	return l
}

func textosDeMapa(m map[string]any) []string {
	var l []string
	for _, v := range m {
		switch x := v.(type) {
		case string:
			l = append(l, x)
		case map[string]any:
			l = append(l, textosDeMapa(x)...)
		}
	}
	return l
}

func variaveisUsadasCondicao(n No) []string {
	var l []string
	for _, r := range n.Ramos {
		for _, rg := range r.Condicoes.Regras {
			if rg.Tipo == RegraVariavel {
				l = append(l, rg.Variavel)
			}
		}
	}
	return l
}

// reEmail valida e-mails nas perguntas (simples, sem espaços, com domínio).
var reEmail = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// EmailValido é usado pelo executor do chatbot.
func EmailValido(s string) bool { return reEmail.MatchString(s) }
