package importacao

import (
	"sync"
	"time"

	"zapdesk/motor/internal/erros"
	"zapdesk/motor/internal/ids"
	"zapdesk/motor/internal/relogio"
)

// ValidadePrevia: a prévia expira em 30 min.
const ValidadePrevia = 30 * time.Minute

// Previa guardada entre a leitura e a importação.
type Previa struct {
	ID       string
	Nome     string
	Planilha Planilha
	ExpiraEm time.Time
}

// Previas é o cache em memória das prévias.
type Previas struct {
	mu      sync.Mutex
	itens   map[string]*Previa
	relogio relogio.Relogio
}

// NovasPrevias cria o cache.
func NovasPrevias(r relogio.Relogio) *Previas {
	return &Previas{itens: map[string]*Previa{}, relogio: r}
}

// Guardar guarda a planilha e devolve a prévia.
func (p *Previas) Guardar(nome string, pl Planilha) *Previa {
	agora := p.relogio.Agora()
	pv := &Previa{ID: ids.NovoEm(agora), Nome: nome, Planilha: pl, ExpiraEm: agora.Add(ValidadePrevia)}
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, x := range p.itens {
		if !agora.Before(x.ExpiraEm) {
			delete(p.itens, id)
		}
	}
	p.itens[pv.ID] = pv
	return pv
}

// Obter devolve a prévia se ainda válida.
func (p *Previas) Obter(id string) (*Previa, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pv, ok := p.itens[id]
	if !ok || !p.relogio.Agora().Before(pv.ExpiraEm) {
		delete(p.itens, id)
		return nil, erros.Novo(erros.NaoEncontrado, "A prévia expirou ou não existe. Envie o arquivo de novo.")
	}
	return pv, nil
}

// Remover descarta a prévia (após a importação).
func (p *Previas) Remover(id string) {
	p.mu.Lock()
	delete(p.itens, id)
	p.mu.Unlock()
}
