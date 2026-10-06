package armazenamento

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"testing/fstest"
)

func TestMigradorBancoNovoEIdempotente(t *testing.T) {
	ctx := context.Background()
	pasta := t.TempDir()
	b, err := Abrir(ctx, pasta)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Fechar()

	aplicadas, err := b.Migrar(ctx, MigracoesPadrao())
	if err != nil {
		t.Fatal(err)
	}
	if len(aplicadas) == 0 || aplicadas[0] != 1 {
		t.Fatalf("esperava aplicar 0001: %v", aplicadas)
	}
	// banco novo: sem backup
	if _, err := os.Stat(b.Caminho + ".bak-0"); err == nil {
		t.Fatal("não deveria haver backup de banco novo")
	}

	aplicadas, err = b.Migrar(ctx, MigracoesPadrao())
	if err != nil || len(aplicadas) != 0 {
		t.Fatalf("reexecução deveria ser vazia: %v %v", aplicadas, err)
	}

	var fk int
	if err := b.E().QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys desligado: %d %v", fk, err)
	}
	var modo string
	b.E().QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&modo)
	if modo != "wal" {
		t.Fatalf("journal_mode = %s", modo)
	}
}

func TestMigradorFazBackup(t *testing.T) {
	ctx := context.Background()
	pasta := t.TempDir()
	b, err := Abrir(ctx, pasta)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Fechar()
	if _, err := b.Migrar(ctx, MigracoesPadrao()); err != nil {
		t.Fatal(err)
	}
	if _, err := b.E().ExecContext(ctx, `INSERT INTO configuracoes (chave, valor) VALUES ('x', '"1"')`); err != nil {
		t.Fatal(err)
	}

	fsys := fstest.MapFS{"9999_extra.sql": {Data: []byte(`CREATE TABLE extra (id TEXT PRIMARY KEY);`)}}
	atual, _ := b.VersaoAtual(ctx)
	aplicadas, err := b.Migrar(ctx, fsys)
	if err != nil || len(aplicadas) != 1 || aplicadas[0] != 9999 {
		t.Fatalf("esperava aplicar 9999: %v %v", aplicadas, err)
	}
	bak := fmt.Sprintf("%s.bak-%d", b.Caminho, atual)
	if _, err := os.Stat(bak); err != nil {
		t.Fatalf("backup não criado: %v", err)
	}
	copia, err := AbrirArquivo(ctx, bak)
	if err != nil {
		t.Fatal(err)
	}
	defer copia.Fechar()
	var valor string
	if err := copia.L().QueryRowContext(ctx, `SELECT valor FROM configuracoes WHERE chave='x'`).Scan(&valor); err != nil {
		t.Fatalf("backup sem dados: %v", err)
	}
}

func TestFTSIgnoraAcentos(t *testing.T) {
	ctx := context.Background()
	b, err := Abrir(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Fechar()
	if _, err := b.Migrar(ctx, MigracoesPadrao()); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := b.E().ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO contas (id, nome, criada_em, atualizada_em) VALUES ('c', 'Conta', 0, 0)`)
	exec(`INSERT INTO conversas (id, conta_id, jid, tipo) VALUES ('v', 'c', 'x@s.whatsapp.net', 'individual')`)
	exec(`INSERT INTO mensagens (id, conta_id, conversa_id, wa_id, remetente_jid, de_mim, tipo, texto, estado, enviada_em)
	      VALUES ('m1', 'c', 'v', 'w1', 'x', 0, 'texto', 'Promoção de ação especial', 'recebida', 1)`)
	exec(`INSERT INTO mensagens (id, conta_id, conversa_id, wa_id, remetente_jid, de_mim, tipo, texto, estado, enviada_em)
	      VALUES ('m2', 'c', 'v', 'w2', 'x', 0, 'texto', 'outra coisa', 'recebida', 2)`)

	var id string
	if err := b.L().QueryRowContext(ctx, `SELECT m.id FROM mensagens_fts f JOIN mensagens m ON m.rid = f.rowid
		WHERE mensagens_fts MATCH ?`, `"acao"`).Scan(&id); err != nil || id != "m1" {
		t.Fatalf("FTS não achou 'acao' em 'ação': %q %v", id, err)
	}
	// atualização e remoção mantêm o índice
	exec(`UPDATE mensagens SET texto = 'nada' WHERE id = 'm1'`)
	var n int
	b.L().QueryRowContext(ctx, `SELECT count(*) FROM mensagens_fts WHERE mensagens_fts MATCH '"acao"'`).Scan(&n)
	if n != 0 {
		t.Fatalf("índice não atualizado: %d", n)
	}
	exec(`DELETE FROM mensagens WHERE id = 'm2'`)
	b.L().QueryRowContext(ctx, `SELECT count(*) FROM mensagens_fts WHERE mensagens_fts MATCH 'outra'`).Scan(&n)
	if n != 0 {
		t.Fatalf("índice não removido: %d", n)
	}
}

// TestMigracao0002SobreBancoComDados aplica a 0002 sobre um banco já na 0001 com dados: cria o
// backup, preserva as mensagens (colunas novas com padrão), cria índices e respeita cascatas.
func TestMigracao0002SobreBancoComDados(t *testing.T) {
	ctx := context.Background()
	b, err := Abrir(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Fechar()
	sql1, _ := os.ReadFile("migracoes/0001_inicial.sql")
	if _, err := b.Migrar(ctx, fstest.MapFS{"0001_inicial.sql": {Data: sql1}}); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := b.E().ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO contas (id, nome, criada_em, atualizada_em) VALUES ('c', 'Conta', 0, 0)`)
	exec(`INSERT INTO conversas (id, conta_id, jid, tipo) VALUES ('v', 'c', 'x@s.whatsapp.net', 'individual')`)
	exec(`INSERT INTO mensagens (id, conta_id, conversa_id, wa_id, remetente_jid, de_mim, tipo, texto, estado, enviada_em)
	      VALUES ('m1', 'c', 'v', 'w1', 'x', 0, 'texto', 'oi', 'recebida', 1)`)
	exec(`INSERT INTO leads (id, telefone, origem, importado_em, atualizado_em) VALUES ('l1', '+5511999990000', 'csv', 0, 0)`)

	aplicadas, err := b.Migrar(ctx, MigracoesPadrao())
	if err != nil || len(aplicadas) != 1 || aplicadas[0] != 2 {
		t.Fatalf("esperava aplicar 0002: %v %v", aplicadas, err)
	}
	if _, err := os.Stat(b.Caminho + ".bak-1"); err != nil {
		t.Fatalf("backup não criado: %v", err)
	}
	var aut sql.NullString
	var primeiro int
	if err := b.L().QueryRowContext(ctx, `SELECT automacao_id, primeiro_contato FROM mensagens WHERE id='m1'`).Scan(&aut, &primeiro); err != nil {
		t.Fatal(err)
	}
	if aut.Valid || primeiro != 0 {
		t.Fatalf("colunas novas com padrão errado: %v %d", aut, primeiro)
	}
	for _, idx := range []string{"sessoes_uma_ativa", "esperas_retomar", "mensagens_automacao", "mensagens_primeiro_contato", "posicoes_etapa"} {
		var n int
		b.L().QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&n)
		if n != 1 {
			t.Fatalf("índice %s ausente", idx)
		}
	}

	// Funil: nome único sem diferenciar maiúsculas; cor #RRGGBB; um lead por funil.
	exec(`INSERT INTO funis (id, nome, ordem, criado_em, atualizado_em) VALUES ('f1', 'Vendas', 0, 0, 0)`)
	if _, err := b.E().ExecContext(ctx, `INSERT INTO funis (id, nome, ordem, criado_em, atualizado_em) VALUES ('f2', 'VENDAS', 1, 0, 0)`); err == nil {
		t.Fatal("nome de funil duplicado aceito")
	}
	exec(`INSERT INTO etapas (id, funil_id, nome, cor, ordem, criada_em) VALUES ('e1', 'f1', 'Novo', '#AABBCC', 0, 0)`)
	if _, err := b.E().ExecContext(ctx, `INSERT INTO etapas (id, funil_id, nome, cor, ordem, criada_em) VALUES ('e2', 'f1', 'x', 'azul', 1, 0)`); err == nil {
		t.Fatal("cor inválida aceita")
	}
	if _, err := b.E().ExecContext(ctx, `INSERT INTO etapas (id, funil_id, nome, cor, ordem, criada_em) VALUES ('e3', 'f1', 'NOVO', '#000000', 1, 0)`); err == nil {
		t.Fatal("nome de etapa duplicado no funil aceito")
	}
	exec(`INSERT INTO posicoes_funil (lead_id, funil_id, etapa_id, desde) VALUES ('l1', 'f1', 'e1', 0)`)
	if _, err := b.E().ExecContext(ctx, `INSERT INTO posicoes_funil (lead_id, funil_id, etapa_id, desde) VALUES ('l1', 'f1', 'e1', 1)`); err == nil {
		t.Fatal("lead duas vezes no mesmo funil")
	}
	exec(`INSERT INTO historico_funil (id, lead_id, funil_id, etapa_destino_id, etapa_destino_nome, origem, em) VALUES ('h1', 'l1', 'f1', 'e1', 'Novo', 'app', 0)`)

	// Automação + sessão única ativa por conversa + espera com chave única.
	exec(`INSERT INTO automacoes (id, tipo, nome, criada_em, atualizada_em) VALUES ('a1', 'chatbot', 'Bot', 0, 0)`)
	if _, err := b.E().ExecContext(ctx, `INSERT INTO automacoes (id, tipo, nome, criada_em, atualizada_em) VALUES ('a2', 'outro', 'X', 0, 0)`); err == nil {
		t.Fatal("tipo de automação inválido aceito")
	}
	exec(`INSERT INTO sessoes_chatbot (id, automacao_id, conversa_id, versao, definicao, no_atual, expira_em, iniciada_em, atualizada_em)
	      VALUES ('s1', 'a1', 'v', 1, '{}', 'n1', 0, 0, 0)`)
	if _, err := b.E().ExecContext(ctx, `INSERT INTO sessoes_chatbot (id, automacao_id, conversa_id, versao, definicao, no_atual, expira_em, iniciada_em, atualizada_em)
	      VALUES ('s2', 'a1', 'v', 1, '{}', 'n1', 0, 0, 0)`); err == nil {
		t.Fatal("duas sessões ativas na mesma conversa")
	}
	exec(`UPDATE sessoes_chatbot SET estado='concluida' WHERE id='s1'`)
	exec(`INSERT INTO sessoes_chatbot (id, automacao_id, conversa_id, versao, definicao, no_atual, expira_em, iniciada_em, atualizada_em)
	      VALUES ('s2', 'a1', 'v', 1, '{}', 'n1', 0, 0, 0)`)
	exec(`INSERT INTO execucoes (id, automacao_id, automacao_versao, tipo_automacao, origem, estado, iniciada_em)
	      VALUES ('x1', 'a1', 1, 'chatbot', 'gatilho', 'ok', 0)`)
	exec(`INSERT INTO esperas (id, tipo, automacao_id, chave, retomar_em, criada_em) VALUES ('w1', 'agendamento', 'a1', 'agendamento:0', 0, 0)`)
	if _, err := b.E().ExecContext(ctx, `INSERT INTO esperas (id, tipo, automacao_id, chave, retomar_em, criada_em) VALUES ('w2', 'agendamento', 'a1', 'agendamento:0', 0, 0)`); err == nil {
		t.Fatal("chave de espera duplicada")
	}
	exec(`INSERT INTO esperas (id, tipo, automacao_id, execucao_id, retomar_em, criada_em) VALUES ('w3', 'aguardar', 'a1', 'x1', 0, 0)`)
	exec(`INSERT INTO esperas (id, tipo, automacao_id, execucao_id, retomar_em, criada_em) VALUES ('w4', 'aguardar', 'a1', 'x1', 0, 0)`)
	exec(`INSERT INTO pausas_conversa (conversa_id, motivo, criada_em) VALUES ('v', 'humano', 0)`)
	exec(`INSERT INTO memoria_automacoes (automacao_id, escopo, chave, valor, atualizada_em) VALUES ('a1', 'global', 'k', '1', 0)`)
	exec(`UPDATE mensagens SET automacao_id = 'a1' WHERE id = 'm1'`)

	// Cascatas: excluir a automação apaga sessões/execuções/esperas/memória e zera automacao_id.
	exec(`DELETE FROM automacoes WHERE id = 'a1'`)
	for _, tab := range []string{"sessoes_chatbot", "execucoes", "esperas", "memoria_automacoes"} {
		var n int
		b.L().QueryRowContext(ctx, `SELECT count(*) FROM `+tab).Scan(&n)
		if n != 0 {
			t.Fatalf("cascata não apagou %s (%d)", tab, n)
		}
	}
	b.L().QueryRowContext(ctx, `SELECT automacao_id FROM mensagens WHERE id='m1'`).Scan(&aut)
	if aut.Valid {
		t.Fatal("automacao_id não virou NULL")
	}
	// Excluir o funil apaga etapas, posições e histórico; excluir a conversa apaga a pausa.
	exec(`DELETE FROM funis WHERE id = 'f1'`)
	for _, tab := range []string{"etapas", "posicoes_funil", "historico_funil"} {
		var n int
		b.L().QueryRowContext(ctx, `SELECT count(*) FROM `+tab).Scan(&n)
		if n != 0 {
			t.Fatalf("cascata do funil não apagou %s", tab)
		}
	}
	exec(`DELETE FROM conversas WHERE id = 'v'`)
	var n int
	b.L().QueryRowContext(ctx, `SELECT count(*) FROM pausas_conversa`).Scan(&n)
	if n != 0 {
		t.Fatal("pausa sobreviveu à conversa")
	}
}
