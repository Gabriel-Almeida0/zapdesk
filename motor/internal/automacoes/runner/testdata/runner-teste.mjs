// Runner de teste do protocolo motor ↔ runner (lado runner simplificado) usado por pool_test.go.
import { createInterface } from 'node:readline';

let proxId = 0;
const pendentes = new Map();
let segredos = {};
let ultimoErro = null;
const cancelados = [];

function enviar(obj) {
  process.stdout.write(JSON.stringify({ jsonrpc: '2.0', ...obj }) + '\n');
}
function requisitar(method, params) {
  const id = ++proxId;
  return new Promise((resolve, reject) => {
    pendentes.set(id, { resolve, reject });
    enviar({ id, method, params });
  });
}
function responder(id, result) { enviar({ id, result }); }
function erro(id, code, message, data) { enviar({ id, error: { code, message, data } }); }

async function executar(id, p) {
  if (p.handler !== 'aoExecutar') return erro(id, 2003, `Handler ${p.handler} não exportado.`);
  const a = p.argumento || {};
  const ex = p.execucao_id;
  switch (a.acao) {
    case 'retornar': return responder(id, { retorno: a.valor ?? null });
    case 'ctx': {
      enviar({ method: 'log', params: { execucao_id: ex, nivel: 'info', texto: 'chamando ctx', em: new Date().toISOString() } });
      enviar({ method: 'http', params: { execucao_id: ex, metodo: 'GET', url_sem_query: 'https://x.test/a', status: 200, duracao_ms: 5, erro: null } });
      try {
        const r = await requisitar('ctx.teste', { execucao_id: ex, x: 1 });
        return responder(id, { retorno: r });
      } catch (e) { return responder(id, { retorno: { erro: e.code } }); }
    }
    case 'ctx_tarde':
      setTimeout(() => requisitar('ctx.teste', { execucao_id: ex }).then(() => { ultimoErro = 0; }, (e) => { ultimoErro = e.code; }), 50);
      return responder(id, { retorno: 'ok' });
    case 'ultimo_erro': return responder(id, { retorno: ultimoErro });
    case 'loop': while (true) { /* trava o laço de eventos */ }
    case 'oom': { const l = []; while (true) l.push(new Array(1e5).fill(Math.random())); }
    case 'sair': process.exit(3);
    case 'stderr': process.stderr.write('linha de erro 1\nlinha de erro 2\n'); setTimeout(() => process.exit(5), 20); return;
    case 'erro': return erro(id, 2002, 'falhou', { nome: 'TypeError', mensagem: 'falhou', stack: 'TypeError: falhou\n    at index.ts:3:1' });
    case 'dormir': setTimeout(() => responder(id, { retorno: 'acordei' }), a.ms || 100); return;
    case 'ambiente': return responder(id, { retorno: { execArgv: process.execArgv, env: Object.keys(process.env), tz: process.env.TZ } });
    case 'pid': return responder(id, { retorno: process.pid });
    case 'segredos': return responder(id, { retorno: segredos });
    case 'cancelados': return responder(id, { retorno: cancelados });
    case 'linha_grande':
      try { await requisitar('ctx.teste', { execucao_id: ex, lixo: 'x'.repeat(1100000) }); return responder(id, { retorno: 'aceito' }); }
      catch (e) { return responder(id, { retorno: 'rejeitado:' + e.code }); }
    default: return erro(id, -32602, 'ação desconhecida');
  }
}

const rl = createInterface({ input: process.stdin, crlfDelay: Infinity });
rl.on('line', (linha) => {
  if (!linha.trim()) return;
  const m = JSON.parse(linha);
  if (m.method === undefined) {
    const p = pendentes.get(m.id);
    if (!p) return;
    pendentes.delete(m.id);
    if (m.error) p.reject(m.error); else p.resolve(m.result);
    return;
  }
  switch (m.method) {
    case 'inicializar':
      if (String(m.params.bundle).includes('invalido')) return erro(m.id, 2001, 'bundle inválido', { stack: 'x' });
      segredos = m.params.segredos;
      return responder(m.id, { handlers: ['aoExecutar'] });
    case 'executar': executar(m.id, m.params); return;
    case 'cancelar': cancelados.push(m.params.execucao_id); return;
    case 'ping': return responder(m.id, { ok: true });
    case 'encerrar': responder(m.id, {}); setTimeout(() => process.exit(0), 10); return;
  }
});
rl.on('close', () => process.exit(0));
enviar({ method: 'pronto', params: { versao_runner: 'teste', protocolo: 1, node: process.version } });
