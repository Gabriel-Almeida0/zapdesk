-- Migração inicial do ZapDesk (data-model.md). Datas em milissegundos Unix UTC; booleanos 0/1;
-- JSON em texto.

CREATE TABLE contas (
    id            TEXT PRIMARY KEY,
    jid           TEXT,
    telefone      TEXT,
    nome          TEXT NOT NULL CHECK (length(nome) BETWEEN 1 AND 60),
    estado        TEXT NOT NULL DEFAULT 'desconectada'
                  CHECK (estado IN ('conectando', 'conectada', 'desconectada', 'banida')),
    sincronizando INTEGER NOT NULL DEFAULT 0,
    criada_em     INTEGER NOT NULL,
    atualizada_em INTEGER NOT NULL
);

CREATE TABLE leads (
    id            TEXT PRIMARY KEY,
    telefone      TEXT NOT NULL UNIQUE,
    nome          TEXT,
    campos        TEXT NOT NULL DEFAULT '{}',
    origem        TEXT NOT NULL CHECK (origem IN ('csv', 'colado', 'contatos', 'mcp')),
    tem_whatsapp  INTEGER,
    importado_em  INTEGER NOT NULL,
    atualizado_em INTEGER NOT NULL
);
CREATE INDEX leads_importado ON leads (importado_em DESC, id DESC);

CREATE TABLE contatos (
    id            TEXT PRIMARY KEY,
    conta_id      TEXT NOT NULL REFERENCES contas (id) ON DELETE CASCADE,
    jid           TEXT NOT NULL,
    telefone      TEXT,
    nome          TEXT,
    nome_push     TEXT,
    notas         TEXT CHECK (notas IS NULL OR length(notas) <= 10000),
    lead_id       TEXT REFERENCES leads (id) ON DELETE SET NULL,
    criado_em     INTEGER NOT NULL,
    atualizado_em INTEGER NOT NULL,
    UNIQUE (conta_id, jid)
);
CREATE INDEX contatos_telefone ON contatos (conta_id, telefone);
CREATE INDEX contatos_lead ON contatos (lead_id);

CREATE TABLE etiquetas (
    id        TEXT PRIMARY KEY,
    nome      TEXT NOT NULL UNIQUE COLLATE NOCASE CHECK (length(nome) BETWEEN 1 AND 30),
    cor       TEXT NOT NULL,
    criada_em INTEGER NOT NULL
);

CREATE TABLE contato_etiquetas (
    contato_id  TEXT NOT NULL REFERENCES contatos (id) ON DELETE CASCADE,
    etiqueta_id TEXT NOT NULL REFERENCES etiquetas (id) ON DELETE CASCADE,
    PRIMARY KEY (contato_id, etiqueta_id)
);
CREATE INDEX contato_etiquetas_etiqueta ON contato_etiquetas (etiqueta_id);

CREATE TABLE conversas (
    id                     TEXT PRIMARY KEY,
    conta_id               TEXT NOT NULL REFERENCES contas (id) ON DELETE CASCADE,
    jid                    TEXT NOT NULL,
    tipo                   TEXT NOT NULL CHECK (tipo IN ('individual', 'grupo')),
    nome                   TEXT,
    contato_id             TEXT REFERENCES contatos (id) ON DELETE SET NULL,
    nao_lidas              INTEGER NOT NULL DEFAULT 0 CHECK (nao_lidas >= 0),
    ultima_mensagem_em     INTEGER,
    ultima_mensagem_resumo TEXT,
    UNIQUE (conta_id, jid)
);
CREATE INDEX conversas_recentes ON conversas (conta_id, ultima_mensagem_em DESC);
CREATE INDEX conversas_contato ON conversas (contato_id);

CREATE TABLE arquivos (
    id         TEXT PRIMARY KEY,
    nome       TEXT NOT NULL,
    mimetype   TEXT NOT NULL,
    tamanho    INTEGER NOT NULL,
    tipo_midia TEXT NOT NULL CHECK (tipo_midia IN ('imagem', 'video', 'audio', 'documento', 'figurinha')),
    caminho    TEXT NOT NULL,
    criado_em  INTEGER NOT NULL
);

CREATE TABLE templates (
    id            TEXT PRIMARY KEY,
    nome          TEXT NOT NULL UNIQUE COLLATE NOCASE CHECK (length(nome) BETWEEN 1 AND 60),
    texto         TEXT NOT NULL CHECK (length(texto) BETWEEN 1 AND 4096),
    arquivo_id    TEXT REFERENCES arquivos (id) ON DELETE SET NULL,
    criado_em     INTEGER NOT NULL,
    atualizado_em INTEGER NOT NULL
);

CREATE TABLE disparos (
    id                   TEXT PRIMARY KEY,
    conta_id             TEXT NOT NULL REFERENCES contas (id) ON DELETE CASCADE,
    nome                 TEXT NOT NULL CHECK (length(nome) BETWEEN 1 AND 80),
    mensagem             TEXT NOT NULL CHECK (length(mensagem) BETWEEN 1 AND 4096),
    arquivo_id           TEXT REFERENCES arquivos (id) ON DELETE SET NULL,
    intervalo_min_s      INTEGER NOT NULL CHECK (intervalo_min_s >= 1),
    intervalo_max_s      INTEGER NOT NULL CHECK (intervalo_max_s >= intervalo_min_s),
    limite_por_hora      INTEGER CHECK (limite_por_hora IS NULL OR limite_por_hora > 0),
    limite_por_dia       INTEGER CHECK (limite_por_dia IS NULL OR limite_por_dia > 0),
    pausa_a_cada         INTEGER CHECK (pausa_a_cada IS NULL OR pausa_a_cada > 0),
    pausa_duracao_s      INTEGER CHECK (pausa_duracao_s IS NULL OR pausa_duracao_s > 0),
    inicio_em            INTEGER,
    janela_inicio        TEXT,
    janela_fim           TEXT,
    falhas_seguidas_max  INTEGER NOT NULL DEFAULT 10 CHECK (falhas_seguidas_max >= 0),
    valores_padrao       TEXT NOT NULL DEFAULT '{}',
    estado               TEXT NOT NULL DEFAULT 'rascunho'
                         CHECK (estado IN ('rascunho', 'agendado', 'enviando', 'fora_da_janela',
                                           'pausado', 'concluido', 'cancelado')),
    motivo_pausa         TEXT CHECK (motivo_pausa IS NULL OR motivo_pausa IN ('usuario', 'app_fechado',
                                     'motor_reiniciado', 'conta_desconectada', 'conta_banida',
                                     'falhas_seguidas')),
    falhas_seguidas      INTEGER NOT NULL DEFAULT 0,
    origem               TEXT NOT NULL CHECK (origem IN ('app', 'mcp')),
    proximo_envio_em     INTEGER,
    ultimo_envio_em      INTEGER,
    enviados_desde_pausa INTEGER NOT NULL DEFAULT 0,
    fila_desde           INTEGER,
    criado_em            INTEGER NOT NULL,
    iniciado_em          INTEGER,
    concluido_em         INTEGER,
    cancelado_em         INTEGER,
    CHECK ((janela_inicio IS NULL) = (janela_fim IS NULL)),
    CHECK ((pausa_a_cada IS NULL) = (pausa_duracao_s IS NULL))
);
CREATE INDEX disparos_conta_estado ON disparos (conta_id, estado);
CREATE INDEX disparos_criado ON disparos (criado_em DESC, id DESC);

CREATE TABLE destinatarios (
    id             TEXT PRIMARY KEY,
    disparo_id     TEXT NOT NULL REFERENCES disparos (id) ON DELETE CASCADE,
    lead_id        TEXT NOT NULL REFERENCES leads (id),
    ordem          INTEGER NOT NULL,
    telefone       TEXT NOT NULL,
    nome           TEXT,
    jid            TEXT,
    variaveis      TEXT NOT NULL DEFAULT '{}',
    estado         TEXT NOT NULL DEFAULT 'pendente'
                   CHECK (estado IN ('pendente', 'enviando', 'enviado', 'entregue', 'lido', 'falhou', 'respondeu')),
    motivo_falha   TEXT,
    mensagem_wa_id TEXT,
    enviando_em    INTEGER,
    enviado_em     INTEGER,
    entregue_em    INTEGER,
    lido_em        INTEGER,
    respondeu_em   INTEGER,
    falhou_em      INTEGER,
    UNIQUE (disparo_id, lead_id)
);
CREATE INDEX destinatarios_estado ON destinatarios (disparo_id, estado);
CREATE INDEX destinatarios_ordem ON destinatarios (disparo_id, ordem);
CREATE INDEX destinatarios_telefone ON destinatarios (telefone, estado);
CREATE INDEX destinatarios_wa ON destinatarios (mensagem_wa_id);
CREATE INDEX destinatarios_lead ON destinatarios (lead_id, enviado_em);

-- rid é a chave de linha estável usada pelo FTS (o rowid implícito pode mudar num VACUUM).
CREATE TABLE mensagens (
    rid                    INTEGER PRIMARY KEY,
    id                     TEXT NOT NULL UNIQUE,
    conta_id               TEXT NOT NULL REFERENCES contas (id) ON DELETE CASCADE,
    conversa_id            TEXT NOT NULL REFERENCES conversas (id) ON DELETE CASCADE,
    wa_id                  TEXT NOT NULL,
    remetente_jid          TEXT NOT NULL,
    remetente_nome         TEXT,
    de_mim                 INTEGER NOT NULL,
    tipo                   TEXT NOT NULL
                           CHECK (tipo IN ('texto', 'imagem', 'video', 'audio', 'documento', 'figurinha', 'sistema')),
    texto                  TEXT,
    midia                  TEXT,
    midia_caminho          TEXT,
    arquivo_id             TEXT REFERENCES arquivos (id) ON DELETE SET NULL,
    citacao_wa_id          TEXT,
    citacao_resumo         TEXT,
    citacao_remetente_nome TEXT,
    editada                INTEGER NOT NULL DEFAULT 0,
    apagada                INTEGER NOT NULL DEFAULT 0,
    estado                 TEXT NOT NULL
                           CHECK (estado IN ('pendente', 'enviada', 'entregue', 'lida', 'falhou', 'recebida')),
    erro                   TEXT,
    disparo_id             TEXT REFERENCES disparos (id) ON DELETE SET NULL,
    enviada_em             INTEGER NOT NULL,
    UNIQUE (conversa_id, wa_id)
);
CREATE INDEX mensagens_conversa ON mensagens (conversa_id, enviada_em DESC, rid DESC);
CREATE INDEX mensagens_remetente ON mensagens (conta_id, remetente_jid, enviada_em);
CREATE INDEX mensagens_wa ON mensagens (conta_id, wa_id);
CREATE INDEX mensagens_disparo ON mensagens (disparo_id);

CREATE VIRTUAL TABLE mensagens_fts USING fts5 (
    texto,
    content = 'mensagens',
    content_rowid = 'rid',
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER mensagens_fts_ins AFTER INSERT ON mensagens BEGIN
    INSERT INTO mensagens_fts (rowid, texto) VALUES (new.rid, new.texto);
END;
CREATE TRIGGER mensagens_fts_del AFTER DELETE ON mensagens BEGIN
    INSERT INTO mensagens_fts (mensagens_fts, rowid, texto) VALUES ('delete', old.rid, old.texto);
END;
CREATE TRIGGER mensagens_fts_upd AFTER UPDATE OF texto ON mensagens BEGIN
    INSERT INTO mensagens_fts (mensagens_fts, rowid, texto) VALUES ('delete', old.rid, old.texto);
    INSERT INTO mensagens_fts (rowid, texto) VALUES (new.rid, new.texto);
END;

CREATE TABLE reacoes (
    mensagem_id   TEXT NOT NULL REFERENCES mensagens (id) ON DELETE CASCADE,
    remetente_jid TEXT NOT NULL,
    emoji         TEXT NOT NULL,
    de_mim        INTEGER NOT NULL DEFAULT 0,
    em            INTEGER NOT NULL,
    PRIMARY KEY (mensagem_id, remetente_jid)
);

CREATE TABLE status_contatos (
    id            TEXT PRIMARY KEY,
    conta_id      TEXT NOT NULL REFERENCES contas (id) ON DELETE CASCADE,
    wa_id         TEXT NOT NULL,
    contato_jid   TEXT NOT NULL,
    contato_nome  TEXT,
    tipo          TEXT NOT NULL CHECK (tipo IN ('texto', 'imagem', 'video')),
    texto         TEXT,
    midia         TEXT,
    midia_caminho TEXT,
    publicado_em  INTEGER NOT NULL,
    UNIQUE (conta_id, wa_id)
);
CREATE INDEX status_recentes ON status_contatos (conta_id, publicado_em DESC);

CREATE TABLE configuracoes (
    chave TEXT PRIMARY KEY,
    valor TEXT NOT NULL
);
