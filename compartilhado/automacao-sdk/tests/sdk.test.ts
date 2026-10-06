import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, expectTypeOf, it } from 'vitest';
import {
  definirAutomacao,
  ehAutomacao,
  ErroAutomacao,
  ErroBloqueado,
  ErroExecucaoEncerrada,
  ErroIA,
  ErroNaoEncontrado,
  ErroPermissao,
  ErroSegredo,
  ErroValidacao,
  handlersDe,
  MARCA_AUTOMACAO,
  type Contexto,
  type EventoAutomacao,
  type Json,
  type MensagemRecebida,
  type RespostaIA,
} from '../src/index.js';

const aqui = dirname(fileURLToPath(import.meta.url));
const raizSdk = join(aqui, '..');

describe('definirAutomacao', () => {
  it('devolve o mesmo objeto marcado com Symbol.for("zapdesk.automacao")', () => {
    const def = { aoExecutar: () => 1 };
    const r = definirAutomacao(def);
    expect(r).toBe(def);
    expect((r as unknown as Record<symbol, unknown>)[Symbol.for('zapdesk.automacao')]).toBe(true);
    expect(MARCA_AUTOMACAO).toBe(Symbol.for('zapdesk.automacao'));
    expect(ehAutomacao(r)).toBe(true);
    expect(Object.keys(r)).toEqual(['aoExecutar']); // marca não enumerável
  });

  it('aceita objeto vazio e lista só os handlers presentes', () => {
    expect(handlersDe(definirAutomacao({}))).toEqual([]);
    const def = definirAutomacao({ aoEvento() {}, aoReceberMensagem: async () => {} });
    expect(handlersDe(def)).toEqual(['aoReceberMensagem', 'aoEvento']);
  });

  it('rejeita handler desconhecido', () => {
    expect(() => definirAutomacao({ aoReceber: () => {} } as never)).toThrow(ErroValidacao);
    try {
      definirAutomacao({ aoReceber: () => {} } as never);
    } catch (e) {
      expect((e as ErroValidacao).campos).toHaveProperty('aoReceber');
      expect((e as Error).message).toContain('Handler desconhecido "aoReceber"');
    }
  });

  it('rejeita handler que não é função e valores que não são objeto', () => {
    expect(() => definirAutomacao({ aoExecutar: 'x' } as never)).toThrow(/precisa ser uma função/);
    expect(() => definirAutomacao(null as never)).toThrow(ErroValidacao);
    expect(() => definirAutomacao([] as never)).toThrow(ErroValidacao);
  });

  it('objetos sem a marca não são automações', () => {
    expect(ehAutomacao({ aoExecutar() {} })).toBe(false);
    expect(ehAutomacao(null)).toBe(false);
  });
});

describe('classes de erro', () => {
  it('têm códigos do runner-protocolo.md, nome e herança', () => {
    const casos: [ErroAutomacao, string, string][] = [
      [new ErroAutomacao('x'), 'erro', 'ErroAutomacao'],
      [new ErroPermissao('enviar'), 'permissao_negada', 'ErroPermissao'],
      [new ErroBloqueado('anti_loop', 'Anti-loop.'), 'bloqueado', 'ErroBloqueado'],
      [new ErroIA('falhou', { status: 529, requestId: 'req_1' }), 'ia_erro', 'ErroIA'],
      [new ErroIA('sem chave', { codigo: 'ia_nao_configurada' }), 'ia_nao_configurada', 'ErroIA'],
      [new ErroSegredo('x'), 'segredo', 'ErroSegredo'],
      [new ErroValidacao('x'), 'validacao', 'ErroValidacao'],
      [new ErroValidacao('x', { codigo: 'limite' }), 'limite', 'ErroValidacao'],
      [new ErroNaoEncontrado('x'), 'nao_encontrado', 'ErroNaoEncontrado'],
      [new ErroExecucaoEncerrada(), 'execucao_encerrada', 'ErroExecucaoEncerrada'],
    ];
    for (const [erro, codigo, nome] of casos) {
      expect(erro).toBeInstanceOf(Error);
      expect(erro).toBeInstanceOf(ErroAutomacao);
      expect(erro.codigo).toBe(codigo);
      expect(erro.name).toBe(nome);
      expect(String(erro.stack)).toContain(nome);
    }
  });

  it('carregam os dados de cada tipo', () => {
    const p = new ErroPermissao('ia');
    expect(p.permissao).toBe('ia');
    expect(p.message).toBe("Permissão 'ia' não declarada em automacao.json");
    expect(new ErroBloqueado('grupo').motivo).toBe('grupo');
    const ia = new ErroIA('x', { status: 529, requestId: 'req_1' });
    expect([ia.status, ia.requestId]).toEqual([529, 'req_1']);
    expect([new ErroIA('x').status, new ErroIA('x').requestId]).toEqual([null, null]);
    expect(new ErroValidacao('x', { campos: { texto: 'vazio' } }).campos).toEqual({ texto: 'vazio' });
    expect(new ErroExecucaoEncerrada().message).toBe('A execução já terminou.');
    expect(new ErroAutomacao('x', { causa: 'y' }).cause).toBe('y');
  });
});

describe('tipos', () => {
  it('handlers recebem ctx e argumentos tipados', () => {
    definirAutomacao({
      async aoReceberMensagem(ctx, msg) {
        expectTypeOf(ctx).toEqualTypeOf<Contexto>();
        expectTypeOf(msg).toEqualTypeOf<MensagemRecebida>();
        expectTypeOf(msg.deMim).toEqualTypeOf<false>();
        expectTypeOf(ctx.ia.gerar).returns.resolves.toEqualTypeOf<RespostaIA>();
        const { categoria } = await ctx.ia.classificar('oi', ['quente', 'frio'] as const);
        expectTypeOf(categoria).toEqualTypeOf<'quente' | 'frio'>();
        const { categoria: c2 } = await ctx.ia.classificar('oi', { a: 'A', b: 'B' });
        expectTypeOf(c2).toEqualTypeOf<'a' | 'b'>();
        expectTypeOf(ctx.segredos.obter).returns.toEqualTypeOf<string>();
        expectTypeOf(ctx.sinal).toEqualTypeOf<AbortSignal>();
      },
      aoExecutar(_ctx, entrada) {
        expectTypeOf(entrada).toEqualTypeOf<Json>();
        return { ok: true };
      },
      aoEvento(_ctx, evento) {
        expectTypeOf(evento).toEqualTypeOf<EventoAutomacao>();
        if (evento.tipo === 'entrou_etapa') expectTypeOf(evento.etapaAnteriorId).toEqualTypeOf<string | null>();
      },
    });
  });

  it('o exemplo "Responder com IA" compila contra o dist/index.d.ts (sem lib DOM nem tipos do Node)', () => {
    const tsc = join(raizSdk, '..', '..', 'node_modules', 'typescript', 'bin', 'tsc');
    const saida = (() => {
      try {
        execFileSync(process.execPath, [tsc, '-p', join(aqui, 'fixtures', 'responder')], { encoding: 'utf8' });
        return '';
      } catch (e) {
        return String((e as { stdout?: string }).stdout ?? e);
      }
    })();
    expect(saida).toBe('');
  });

  it('o dist/index.d.ts é um script de declarações com o módulo e os curingas de texto', () => {
    const dts = readFileSync(join(raizSdk, 'dist', 'index.d.ts'), 'utf8');
    expect(dts.startsWith('/// <reference lib="dom"')).toBe(true);
    expect(dts).toContain("declare module '@zapdesk/automacao' {");
    expect(dts).toContain("declare module '*.md' {");
    expect(dts).toContain("declare module '*.txt' {");
    expect(dts).not.toMatch(/^\s*(import|export)\b[^\n]*from\s+['"]\./m);
    expect(dts).not.toMatch(/^\s+(export\s+)?declare\s/m);
  });
});
