-- Migração da feature 002 — automações (specs/002-automacoes/data-model.md): funis, etapas,
-- posições e histórico do funil; automações, sessões de chatbot, execuções, esperas, pausas de
-- conversa e memória das automações; colunas novas em mensagens. Datas em ms Unix UTC; JSON em
-- texto; booleanos 0/1.

CREATE TABLE funis (
    id            TEXT PRIMARY KEY,
    nome          TEXT NOT NULL UNIQUE COLLATE NOCASE CHECK (length(nome) BETWEEN 1 AND 60),
    ordem         INTEGER NOT NULL CHECK (ordem >= 0),
    criado_em     INTEGER NOT NULL,
    atualizado_em INTEGER NOT NULL
);

CREATE TABLE etapas (
    id        TEXT PRIMARY KEY,
    funil_id  TEXT NOT NULL REFERENCES funis (id) ON DELETE CASCADE,
    nome      TEXT NOT NULL COLLATE NOCASE CHECK (length(nome) BETWEEN 1 AND 40),
    cor       TEXT NOT NULL DEFAULT '#8696A0'
              CHECK (length(cor) = 7 AND substr(cor, 1, 1) = '#'
                     AND NOT substr(cor, 2) GLOB '*[^0-9A-Fa-f]*'),
    ordem     INTEGER NOT NULL CHECK (ordem >= 0),
    criada_em INTEGER NOT NULL,
    UNIQUE (funil_id, nome)
);
CREATE INDEX etapas_funil ON etapas (funil_id, ordem);

CREATE TABLE posicoes_funil (
    lead_id  TEXT NOT NULL REFERENCES leads (id) ON DELETE CASCADE,
    funil_id TEXT NOT NULL REFERENCES funis (id) ON DELETE CASCADE,
    etapa_id TEXT NOT NULL REFERENCES etapas (id) ON DELETE CASCADE,
    desde    INTEGER NOT NULL,
    PRIMARY KEY (lead_id, funil_id)
);
CREATE INDEX posicoes_etapa ON posicoes_funil (funil_id, etapa_id, desde DESC);

CREATE TABLE historico_funil (
    id                 TEXT PRIMARY KEY,
    lead_id            TEXT NOT NULL REFERENCES leads (id) ON DELETE CASCADE,
    funil_id           TEXT NOT NULL REFERENCES funis (id) ON DELETE CASCADE,
    etapa_origem_id    TEXT,
    etapa_origem_nome  TEXT,
    etapa_destino_id   TEXT,
    etapa_destino_nome TEXT,
    origem             TEXT NOT NULL CHECK (origem IN ('app', 'mcp', 'automacao')),
    automacao_id       TEXT,
    execucao_id        TEXT,
    em                 INTEGER NOT NULL
);
CREATE INDEX historico_funil_funil ON historico_funil (funil_id, id DESC);
CREATE INDEX historico_funil_lead ON historico_funil (lead_id, id DESC);

CREATE TABLE automacoes (
    id                TEXT PRIMARY KEY,
    tipo              TEXT NOT NULL CHECK (tipo IN ('fluxo', 'chatbot', 'ia')),
    nome              TEXT NOT NULL CHECK (length(nome) BETWEEN 1 AND 80),
    descricao         TEXT CHECK (descricao IS NULL OR length(descricao) <= 500),
    ativa             INTEGER NOT NULL DEFAULT 0,
    contas            TEXT,
    incluir_grupos    INTEGER NOT NULL DEFAULT 0,
    prioridade        INTEGER NOT NULL DEFAULT 100 CHECK (prioridade BETWEEN 1 AND 1000),
    conta_envio_id    TEXT REFERENCES contas (id) ON DELETE SET NULL,
    gatilhos          TEXT NOT NULL DEFAULT '[]',
    definicao         TEXT,
    limites           TEXT NOT NULL DEFAULT '{}',
    versao            INTEGER NOT NULL DEFAULT 1,
    permissoes        TEXT,
    segredos          TEXT,
    hash_fontes       TEXT,
    hash_compilado    TEXT,
    erros_compilacao  TEXT NOT NULL DEFAULT '[]',
    compilado_em      INTEGER,
    erros_seguidos    INTEGER NOT NULL DEFAULT 0,
    desativada_motivo TEXT CHECK (desativada_motivo IS NULL OR desativada_motivo IN ('erros_seguidos', 'usuario')),
    criada_em         INTEGER NOT NULL,
    atualizada_em     INTEGER NOT NULL
);
CREATE INDEX automacoes_ativas ON automacoes (ativa, prioridade, criada_em);

CREATE TABLE sessoes_chatbot (
    id            TEXT PRIMARY KEY,
    automacao_id  TEXT NOT NULL REFERENCES automacoes (id) ON DELETE CASCADE,
    conversa_id   TEXT NOT NULL REFERENCES conversas (id) ON DELETE CASCADE,
    versao        INTEGER NOT NULL,
    definicao     TEXT NOT NULL,
    no_atual      TEXT NOT NULL,
    variaveis     TEXT NOT NULL DEFAULT '{}',
    tentativas    INTEGER NOT NULL DEFAULT 0,
    estado        TEXT NOT NULL DEFAULT 'ativa'
                  CHECK (estado IN ('ativa', 'concluida', 'humano', 'expirada', 'abortada')),
    motivo        TEXT,
    expira_em     INTEGER NOT NULL,
    iniciada_em   INTEGER NOT NULL,
    atualizada_em INTEGER NOT NULL,
    finalizada_em INTEGER
);
CREATE UNIQUE INDEX sessoes_uma_ativa ON sessoes_chatbot (conversa_id) WHERE estado = 'ativa';
CREATE INDEX sessoes_automacao ON sessoes_chatbot (automacao_id, iniciada_em DESC);

CREATE TABLE execucoes (
    id                 TEXT PRIMARY KEY,
    automacao_id       TEXT NOT NULL REFERENCES automacoes (id) ON DELETE CASCADE,
    automacao_versao   INTEGER NOT NULL,
    tipo_automacao     TEXT NOT NULL CHECK (tipo_automacao IN ('fluxo', 'chatbot', 'ia')),
    gatilho            TEXT NOT NULL DEFAULT '{}',
    origem             TEXT NOT NULL
                       CHECK (origem IN ('gatilho', 'manual_app', 'manual_mcp', 'teste', 'fluxo', 'chatbot')),
    origem_execucao_id TEXT,
    cadeia             TEXT NOT NULL DEFAULT '[]',
    conta_id           TEXT,
    conversa_id        TEXT,
    contato_id         TEXT,
    lead_id            TEXT,
    estado             TEXT NOT NULL
                       CHECK (estado IN ('na_fila', 'rodando', 'aguardando', 'ok', 'erro', 'simulacao', 'abortada')),
    simulacao          INTEGER NOT NULL DEFAULT 0,
    passo_atual        INTEGER,
    variaveis          TEXT NOT NULL DEFAULT '{}',
    acoes              TEXT NOT NULL DEFAULT '[]',
    log                TEXT NOT NULL DEFAULT '',
    log_truncado       INTEGER NOT NULL DEFAULT 0,
    erro               TEXT,
    erro_stack         TEXT,
    tokens             TEXT NOT NULL DEFAULT '{"entrada":0,"saida":0,"por_modelo":{}}',
    retorno            TEXT,
    retomar_em         INTEGER,
    iniciada_em        INTEGER NOT NULL,
    finalizada_em      INTEGER,
    duracao_ms         INTEGER,
    motivo             TEXT
);
CREATE INDEX execucoes_automacao ON execucoes (automacao_id, iniciada_em DESC);
CREATE INDEX execucoes_conversa ON execucoes (conversa_id, iniciada_em DESC);
CREATE INDEX execucoes_estado ON execucoes (estado);
CREATE INDEX execucoes_recentes ON execucoes (iniciada_em DESC, id DESC);

CREATE TABLE esperas (
    id           TEXT PRIMARY KEY,
    tipo         TEXT NOT NULL CHECK (tipo IN ('aguardar', 'sem_resposta', 'agendamento', 'agendar', 'expirar_sessao')),
    automacao_id TEXT NOT NULL REFERENCES automacoes (id) ON DELETE CASCADE,
    execucao_id  TEXT REFERENCES execucoes (id) ON DELETE CASCADE,
    sessao_id    TEXT REFERENCES sessoes_chatbot (id) ON DELETE CASCADE,
    conversa_id  TEXT,
    referencia   TEXT,
    chave        TEXT,
    retomar_em   INTEGER NOT NULL,
    dados        TEXT NOT NULL DEFAULT '{}',
    criada_em    INTEGER NOT NULL,
    UNIQUE (automacao_id, chave)
);
CREATE INDEX esperas_retomar ON esperas (retomar_em);
CREATE INDEX esperas_conversa ON esperas (conversa_id, tipo);

CREATE TABLE pausas_conversa (
    conversa_id  TEXT PRIMARY KEY REFERENCES conversas (id) ON DELETE CASCADE,
    motivo       TEXT NOT NULL CHECK (motivo IN ('humano', 'anti_loop', 'manual')),
    ate          INTEGER,
    automacao_id TEXT,
    criada_em    INTEGER NOT NULL
);

CREATE TABLE memoria_automacoes (
    automacao_id  TEXT NOT NULL REFERENCES automacoes (id) ON DELETE CASCADE,
    escopo        TEXT NOT NULL,
    chave         TEXT NOT NULL CHECK (length(chave) BETWEEN 1 AND 200),
    valor         TEXT NOT NULL CHECK (length(CAST(valor AS BLOB)) <= 65536),
    atualizada_em INTEGER NOT NULL,
    PRIMARY KEY (automacao_id, escopo, chave)
);

ALTER TABLE mensagens ADD COLUMN automacao_id TEXT REFERENCES automacoes (id) ON DELETE SET NULL;
ALTER TABLE mensagens ADD COLUMN primeiro_contato INTEGER NOT NULL DEFAULT 0;
CREATE INDEX mensagens_automacao ON mensagens (conversa_id, automacao_id, enviada_em);
CREATE INDEX mensagens_primeiro_contato ON mensagens (conta_id, primeiro_contato, enviada_em);
-- Cards do Kanban procuram o contato/conversa mais recente de um telefone em qualquer conta.
CREATE INDEX contatos_so_telefone ON contatos (telefone);
