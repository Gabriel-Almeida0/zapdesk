# Contrato interno — interface `ClienteWhatsApp` (motor)

Pacote: `motor/internal/whatsapp`. Implementações: `motor/internal/whatsapp/whatsmeow` (real,
único lugar que importa `go.mau.fi/whatsmeow`) e `motor/internal/whatsapp/falso` (memória).
Constituição IV. O domínio só conhece os tipos abaixo (nunca `types.JID`, `waE2E.*`, `events.*`).

```go
package whatsapp

// Fabrica cria um cliente por conta; a implementação real abre sessoes/<contaID>.db.
type Fabrica interface {
    Abrir(ctx context.Context, contaID string) (Cliente, error)
    Remover(ctx context.Context, contaID string) error // logout + apaga sessão
}

type Cliente interface {
    // Conexão
    Conectar(ctx context.Context) error          // restaura sessão ou inicia QR
    Desconectar()
    Pareado() bool                               // tem sessão salva
    CanalQR(ctx context.Context) (<-chan EventoQR, error)
    Eventos() <-chan Evento                      // todos os eventos normalizados (fechado ao desconectar)
    Proprio() (JID, bool)                        // JID/telefone da conta

    // Consulta
    TemWhatsApp(ctx context.Context, telefoneE164 string) (JID, bool, error)
    Grupos(ctx context.Context) ([]InfoGrupo, error)
    Contatos(ctx context.Context) ([]InfoContato, error)

    // Envio (retornam o id do WhatsApp e o horário do servidor)
    EnviarTexto(ctx context.Context, para JID, texto string, citar *Citacao) (Enviada, error)
    EnviarMidia(ctx context.Context, para JID, m MidiaEnvio, citar *Citacao) (Enviada, error)
    Reagir(ctx context.Context, chat, remetente JID, waID, emoji string) error
    Editar(ctx context.Context, chat JID, waID, novoTexto string) error
    Apagar(ctx context.Context, chat, remetente JID, waID string) error
    MarcarLida(ctx context.Context, chat, remetente JID, waIDs []string, em time.Time) error

    // Mídia
    Baixar(ctx context.Context, chave ChaveDownload) (io.ReadCloser, error)
}

type JID struct{ Usuario, Servidor string } // sempre telefone quando conhecido (LID resolvido no adaptador)

type EventoQR struct {
    Tipo   string // "codigo" | "sucesso" | "expirado" | "erro"
    Codigo string
    Expira time.Duration
    Erro   error
}

// Evento é uma união: exatamente um campo não nulo.
type Evento struct {
    Estado      *EventoEstado      // conectada | desconectada (logout) | banida | rede_caiu | rede_voltou | substituida
    Mensagem    *MensagemRecebida  // inclui mensagens enviadas pelo próprio celular (DeMim=true) e status (Chat=status@broadcast)
    Recibo      *Recibo            // entregue | lido | tocado, com lista de waIDs
    Reacao      *Reacao
    Edicao      *Edicao
    Revogacao   *Revogacao
    Historico   *LoteHistorico     // conversas + mensagens do history sync (já convertidas)
    Contato     *InfoContato       // push name / agenda
}

type MensagemRecebida struct {
    WaID, TipoMsg, Texto string // TipoMsg: texto|imagem|video|audio|documento|figurinha|sistema
    Chat, Remetente      JID
    DeMim, Grupo         bool
    Midia                *MidiaInfo // metadados + ChaveDownload
    Citacao              *Citacao
    Em                   time.Time
}

type MidiaEnvio struct {
    Tipo      string // imagem|video|audio|documento|figurinha
    Leitor    io.Reader
    Tamanho   int64
    Mimetype  string
    NomeArq   string
    Legenda   string
    Voz       bool   // PTT (áudio ogg/opus)
}

type Enviada struct { WaID string; Em time.Time }
```

Tipos auxiliares (`EventoEstado`, `Recibo`, `Reacao`, `Edicao`, `Revogacao`, `LoteHistorico`,
`InfoGrupo`, `InfoContato`, `MidiaInfo`, `ChaveDownload`, `Citacao`) são structs simples de dados
definidos no mesmo pacote; `ChaveDownload` é serializável em JSON (guardada em
`mensagens.midia.chave_download`).

Erros sentinela: `ErrSemWhatsApp`, `ErrDesconectado`, `ErrBanido`, `ErrForaDoPrazo`,
`ErrMidiaExpirada` e (acréscimo da implementação) `ErrEstadoIncerto` — tempo esgotado depois de
enviar: a mensagem pode ter saído, então o disparo marca `falhou` ("Estado incerto após
interrupção") em vez de reenviar. `MensagemRecebida` ganhou `NomeRemetente` (push name) e
`NomeChat` (assunto do grupo). O adaptador real traduz erros do whatsmeow para esses sentinelas; erros de
rede transitórios são `ErrDesconectado` (o disparo aguarda, não marca `falhou`).

Relógio: `motor/internal/relogio` expõe `type Relogio interface { Agora() time.Time;
Esperar(ctx, ate time.Time) error }` — real e controlável (modo falso/testes).
