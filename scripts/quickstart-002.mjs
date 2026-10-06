// T133 — roda o §2 de specs/002-automacoes/quickstart.md contra o motor real (WhatsApp falso, IA
// falsa, runner via node) numa pasta temporária e imprime OK/FALHA por passo.
// Pré-requisito: npm run motor:compilar && npm run runner:compilar. Uso: node scripts/quickstart-002.mjs
import { spawn } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createInterface } from 'node:readline';
const Z = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const TOKEN = 'quickstart-002-token-com-mais-de-32-caracteres';
let pasta = mkdtempSync(join(tmpdir(), 'zd-qs-'));
let motor, BASE;
const resultados = [];
const ok = (nome, cond, extra = '') => { resultados.push([nome, !!cond, extra]); console.log(`${cond ? 'OK   ' : 'FALHA'} ${nome} ${extra}`); };
async function subir() {
  motor = spawn(`${Z}/motor/bin/zapdesk-motor`, ['--pasta-dados', pasta, '--whatsapp=falso', '--ia=falsa', '--sem-stdin', '--runner-exec', process.execPath, '--runner-script', `${Z}/automacao/runner/dist/zapdesk-runner.mjs`], { env: { ...process.env, ZAPDESK_TOKEN: TOKEN }, stdio: ['ignore', 'pipe', 'ignore'] });
  const rl = createInterface({ input: motor.stdout });
  const porta = await new Promise((r) => rl.on('line', (l) => { try { const j = JSON.parse(l); if (j.evento === 'pronto') r(j.porta); } catch {} }));
  BASE = `http://127.0.0.1:${porta}/v1`;
}
async function parar() { motor.kill('SIGTERM'); await new Promise((r) => motor.once('exit', r)); }
async function api(m, c, corpo) {
  const r = await fetch(BASE + c, { method: m, headers: { authorization: `Bearer ${TOKEN}`, 'content-type': 'application/json' }, body: corpo === undefined ? undefined : JSON.stringify(corpo) });
  const t = await r.text(); let j = null; try { j = JSON.parse(t); } catch {}
  return { s: r.status, j };
}
const esperar = (ms) => new Promise((r) => setTimeout(r, ms));
async function ate(fn, ms = 10000) { const fim = Date.now() + ms; while (Date.now() < fim) { const v = await fn(); if (v) return v; await esperar(100); } return null; }
const enviadasPara = async (tel) => (await api('GET', '/falso/enviadas')).j.filter((e) => e.telefone === tel);
const execs = async (id) => (await api('GET', `/automacoes/${id}/execucoes?limite=200`)).j.itens;
const receber = (c, de, texto) => api('POST', `/falso/contas/${c}/mensagem-recebida`, { de, texto, nome: 'Contato ' + de.slice(-4) });
const fluxo = (nome, gatilhos, ...acoes) => ({ tipo: 'fluxo', nome, gatilhos, definicao: { versao: 1, acoes } });
async function criarAtivar(corpo) { const a = await api('POST', '/automacoes', corpo); if (a.s !== 201) console.log(JSON.stringify(a.j)); await api('POST', `/automacoes/${a.j.id}/ativar`, {}); return a.j; }
async function desativarTodas() { for (const a of (await api('GET', '/automacoes')).j) await api('POST', `/automacoes/${a.id}/desativar`, {}); }

await subir();
// 1. conta
const conta = (await api('POST', '/contas', { nome: 'Loja' })).j;
await api('POST', `/falso/contas/${conta.id}/escanear-qr`, { telefone: '+5511900000001', nome: 'Loja' });
ok('1 conta conectada', await ate(async () => (await api('GET', `/contas/${conta.id}`)).j.estado === 'conectada'));
// 2. funil
const f = (await api('POST', '/funis', { nome: 'Prospecção', etapas: [{ nome: 'Novo' }, { nome: 'Qualificando' }, { nome: 'Proposta' }, { nome: 'Fechado' }] })).j;
const c1 = await api('PUT', `/funis/${f.id}/cards`, { telefone: '+5511900000002', etapa_id: f.etapas[0].id });
const c2 = await api('PUT', `/funis/${f.id}/cards`, { telefone: '+5511900000002', etapa_id: f.etapas[1].id });
const hist = (await api('GET', `/funis/${f.id}/historico`)).j.itens;
ok('2 funil: card 201 → 200 e 2 entradas no histórico', c1.s === 201 && c2.s === 200 && hist.length === 2, `(${c1.s}, ${c2.s}, ${hist.length})`);
// 3. fluxo disparo_respondeu
const quente = (await api('POST', '/etiquetas', { nome: 'quente', cor: '#FF8800' })).j;
const bot = (await api('POST', '/automacoes', { tipo: 'chatbot', nome: 'Bot', gatilhos: [{ tipo: 'palavra_chave', palavras: ['zzzbot'] }], definicao: { versao: 1, inicio: 'n1', nao_entendi: 'Não entendi.', max_tentativas: 3, inatividade_min: 30, nos: [{ id: 'n1', tipo: 'inicio', proximo: 'n2' }, { id: 'n2', tipo: 'mensagem', texto: 'Olá do bot!', proximo: 'n3' }, { id: 'n3', tipo: 'fim', mensagem: 'Tchau.' }] } })).j;
const aResp = await criarAtivar(fluxo('Respondeu', [{ tipo: 'disparo_respondeu' }], { tipo: 'adicionar_etiqueta', etiqueta_id: quente.id }, { tipo: 'mover_etapa', funil_id: f.id, etapa_id: f.etapas[1].id }, { tipo: 'iniciar_chatbot', automacao_id: bot.id }));
const imp = (await api('POST', '/leads/importar', { leads: [{ telefone: '+5511900000003', nome: 'Ana' }] })).j;
const d = await api('POST', '/disparos', { conta_id: conta.id, mensagem: 'Oi {nome}!', iniciar: true, destinatarios: { lead_ids: imp.lead_ids }, ritmo: { intervalo_min_s: 30, intervalo_max_s: 60 } });
await ate(async () => (await enviadasPara('+5511900000003')).length > 0);
await receber(conta.id, '+5511900000003', 'oi');
const e3 = await ate(async () => { const e = await execs(aResp.id); return e.length && e[0].estado !== 'rodando' && e[0].estado !== 'na_fila' ? e[0] : null; });
ok('3 fluxo disparo_respondeu: ok com 3 ações', d.s === 201 && e3?.estado === 'ok' && e3.acoes.length === 3 && e3.acoes.every((a) => a.resultado === 'ok'), JSON.stringify(e3?.acoes.map((a) => a.resultado)));
await desativarTodas();
// 4. sem resposta com reinício e expiração
const tpl = (await api('POST', '/templates', { nome: 'Follow-up', texto: 'Ainda tem interesse, {primeiro_nome}?' })).j;
const aSR = await criarAtivar(fluxo('Sem resposta', [{ tipo: 'sem_resposta', apos_s: 7200 }], { tipo: 'enviar_template', template_id: tpl.id }));
await receber(conta.id, '+5511900000004', 'Quanto custa?');
const conv4 = (await api('GET', `/contas/${conta.id}/conversas`)).j.itens.find((c) => c.telefone === '+5511900000004');
await api('POST', `/conversas/${conv4.id}/mensagens`, { texto: 'Custa R$ 50.' });
await ate(async () => (await enviadasPara('+5511900000004')).length === 1);
await api('PUT', '/falso/relogio', { avancar_s: 3600 });
await parar(); await subir(); // motor reiniciado no meio da espera (relógio volta ao real)
await ate(async () => (await api('GET', `/contas/${conta.id}`)).j.estado === 'conectada');
await api('PUT', '/falso/relogio', { avancar_s: 7300 }); await api('POST', '/falso/processar-esperas', {});
await api('PUT', '/falso/relogio', { avancar_s: 3 * 3600 }); await api('POST', '/falso/processar-esperas', {});
const env4 = await enviadasPara('+5511900000004');
ok('4a sem resposta: template sai uma única vez após reinício', env4.length === 2 && env4[1].texto.includes('Ainda tem interesse'), `(${env4.length} enviadas)`);
await api('POST', `/conversas/${conv4.id}/mensagens`, { texto: 'Posso ajudar?' });
await ate(async () => (await enviadasPara('+5511900000004')).length === 3);
await parar(); await subir();
await ate(async () => (await api('GET', `/contas/${conta.id}`)).j.estado === 'conectada');
await api('PUT', '/falso/relogio', { avancar_s: 40 * 3600 /* o relógio falso volta ao real no reinício: 40 h cobre as 5 h já avançadas */ }); await api('POST', '/falso/processar-esperas', {});
const ex4 = await execs(aSR.id);
ok('4b espera vencida há 30 h: abortada "expirada", nada enviado', (await enviadasPara('+5511900000004')).length === 3 && ex4[0].estado === 'abortada' && ex4[0].motivo === 'expirada', `${ex4[0]?.estado}/${ex4[0]?.motivo}`);
await desativarTodas();
// 5. anti-loop e humano
const aEco = await criarAtivar(fluxo('Eco', [{ tipo: 'mensagem_recebida' }], { tipo: 'enviar_texto', texto: 'Recebido: {ultima_mensagem}' }));
for (let i = 1; i <= 12; i++) await receber(conta.id, '+5511900000005', `msg ${i}`);
await esperar(1500);
const conv5 = (await api('GET', `/contas/${conta.id}/conversas`)).j.itens.find((c) => c.telefone === '+5511900000005');
const est5 = (await api('GET', `/conversas/${conv5.id}/automacoes`)).j;
ok('5a anti-loop: 10 automáticas e pausa anti_loop', (await enviadasPara('+5511900000005')).length === 10 && est5.pausa?.motivo === 'anti_loop', `(${(await enviadasPara('+5511900000005')).length})`);
await receber(conta.id, '+5511900000006', 'oi');
await esperar(500);
const conv6 = (await api('GET', `/contas/${conta.id}/conversas`)).j.itens.find((c) => c.telefone === '+5511900000006');
await api('POST', `/conversas/${conv6.id}/mensagens`, { texto: 'Aqui é a Ana.' });
await esperar(500);
ok('5b mensagem manual pela API → pausa humano', (await api('GET', `/conversas/${conv6.id}/automacoes`)).j.pausa?.motivo === 'humano');
await desativarTodas();
// 6. IA
const ia = (await api('POST', '/automacoes/ia', { nome: 'Resp', modelo: 'responder_historico' })).j;
const antes6 = (await api('GET', '/falso/enviadas')).j.length;
const t6 = (await api('POST', `/automacoes/${ia.id}/testar`, { mensagem: { texto: 'Quanto custa?' }, ia_simulada: true })).j;
ok('6a IA: compilação ok, teste em simulação sem enviar', ia.ia?.compilacao_ok && t6.execucao.estado === 'simulacao' && t6.execucao.acoes[0]?.resultado === 'simulada' && (await api('GET', '/falso/enviadas')).j.length === antes6);
await api('POST', `/automacoes/${ia.id}/ativar`, {});
await receber(conta.id, '+5511900000007', 'Quanto custa?');
const r6 = await ate(async () => (await enviadasPara('+5511900000007'))[0]);
ok('6b IA ativa responde "[IA simulada] …"', r6?.texto?.startsWith('[IA simulada]'), r6?.texto);
// 7. isolamento
const br = (await api('POST', '/automacoes/ia', { nome: 'Laço', modelo: 'em_branco' })).j;
const man = JSON.parse((await api('GET', `/automacoes/${br.id}/arquivos/automacao.json`)).j.conteudo);
man.gatilhos = [{ tipo: 'mensagem_recebida' }]; man.limites = { ...(man.limites ?? {}), tempo_s: 5 };
await api('PUT', `/automacoes/${br.id}/arquivos/automacao.json`, { conteudo: JSON.stringify(man, null, 2) });
await api('PUT', `/automacoes/${br.id}/arquivos/index.ts`, { conteudo: "import { definirAutomacao } from '@zapdesk/automacao';\nexport default definirAutomacao({ aoReceberMensagem() { while (true) {} } });\n" });
const at7 = await api('POST', `/automacoes/${br.id}/ativar`, {});
const t0 = Date.now();
await receber(conta.id, '+5511900000008', 'oi');
const e7 = await ate(async () => { const e = await execs(br.id); return e[0] && e[0].estado === 'erro' ? e[0] : null; }, 30000);
const r7 = await ate(async () => (await enviadasPara('+5511900000008'))[0], 30000);
ok('7a laço infinito → erro por tempo limite; a outra IA continua respondendo', at7.s === 200 && e7 && r7, `(${e7?.erro}; ${Date.now() - t0} ms)`);
for (let i = 0; i < 5; i++) { await receber(conta.id, `+55119000001${i}0`, 'oi'); await ate(async () => (await execs(br.id)).filter((e) => e.estado === 'erro').length >= i + 2, 30000); }
const a7 = (await api('GET', `/automacoes/${br.id}`)).j;
ok('7b 5 erros seguidos desativam', a7.ativa === false && a7.desativada_motivo === 'erros_seguidos', `ativa=${a7.ativa} motivo=${a7.desativada_motivo}`);
await api('PUT', `/automacoes/${br.id}/arquivos/index.ts`, { conteudo: "import fs from 'fs';\nimport { definirAutomacao } from '@zapdesk/automacao';\nexport default definirAutomacao({ aoReceberMensagem() { fs.readFileSync('x'); } });\n" });
const c7 = await api('POST', `/automacoes/${br.id}/ativar`, {});
ok('7c import fs → compilacao_falhou "Módulo não permitido: fs"', c7.s === 422 && c7.j.erro.codigo === 'compilacao_falhou' && JSON.stringify(c7.j).includes('Módulo não permitido: fs'));
const semEnv = (await api('POST', '/automacoes/ia', { nome: 'SemEnviar', modelo: 'em_branco' })).j;
const m2 = JSON.parse((await api('GET', `/automacoes/${semEnv.id}/arquivos/automacao.json`)).j.conteudo);
m2.permissoes = ['ler_conversas']; m2.gatilhos = [{ tipo: 'mensagem_recebida' }];
await api('PUT', `/automacoes/${semEnv.id}/arquivos/automacao.json`, { conteudo: JSON.stringify(m2) });
await api('PUT', `/automacoes/${semEnv.id}/arquivos/index.ts`, { conteudo: "import { definirAutomacao } from '@zapdesk/automacao';\nexport default definirAutomacao({ async aoReceberMensagem(ctx) { await ctx.responder('oi'); } });\n" });
const t7 = (await api('POST', `/automacoes/${semEnv.id}/testar`, { mensagem: { texto: 'oi' } })).j;
ok('7d sem permissão enviar → erro de permissão', JSON.stringify(t7).includes("Permissão 'enviar' não declarada em automacao.json"));
// 8. chatbot do formatos.md (sem o nó ia)
const def8 = { versao: 1, inicio: 'n1', nao_entendi: 'Não entendi. Responda com uma das opções.', max_tentativas: 3, inatividade_min: 30, nos: [
  { id: 'n1', tipo: 'inicio', proximo: 'n2' }, { id: 'n2', tipo: 'mensagem', texto: 'Olá, {nome}!', proximo: 'n3' },
  { id: 'n3', tipo: 'menu', texto: 'Como posso ajudar?', opcoes: [{ rotulo: 'Preços', valores: ['preco'], proximo: 'n4' }, { rotulo: 'Falar com vendedor', valores: [], proximo: 'n8' }], mostrar_numeros: true, ao_esgotar: null },
  { id: 'n4', tipo: 'pergunta', texto: 'Qual seu e-mail?', variavel: 'email', validacao: { tipo: 'email', padrao: null, mensagem_erro: 'E-mail inválido. Tente de novo.' }, proximo: 'n9', ao_esgotar: null },
  { id: 'n8', tipo: 'humano', mensagem: 'Vou chamar um atendente.' }, { id: 'n9', tipo: 'fim', mensagem: 'Obrigado, {email}!' }] };
const b8 = (await api('POST', '/automacoes', { tipo: 'chatbot', nome: 'Bot 8', gatilhos: [{ tipo: 'palavra_chave', palavras: ['orçamento'] }], definicao: def8 })).j;
let s8 = (await api('POST', `/automacoes/${b8.id}/simulador`, {})).j;
let u; for (const t of ['1', 'x', 'x', 'x']) u = (await api('POST', `/simulador/${s8.simulacao_id}/mensagens`, { texto: t })).j;
ok('8a simulador: 3 respostas inválidas → humano', /humano/.test(u.estado) || JSON.stringify(u).includes('atendente'), u.estado);
s8 = (await api('POST', `/automacoes/${b8.id}/simulador`, {})).j;
await api('POST', `/simulador/${s8.simulacao_id}/mensagens`, { texto: '1' });
u = (await api('POST', `/simulador/${s8.simulacao_id}/mensagens`, { texto: 'ana@exemplo.com' })).j;
ok('8b e-mail válido preenche {email}', u.variaveis?.email === 'ana@exemplo.com', JSON.stringify(u.variaveis));
await parar(); rmSync(pasta, { recursive: true, force: true });
const falhas = resultados.filter((r) => !r[1]).length;
console.log(`\n${resultados.length - falhas}/${resultados.length} passos ok`);
process.exit(falhas ? 1 : 0);
