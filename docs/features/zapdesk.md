# Feature: ZapDesk — WhatsApp desktop local com disparo em massa e MCP

> **Status:** Especificação pronta para implementação
> **Slug:** `zapdesk`
> **Criado em:** 2026-09-27
> **Autor da spec:** Claude (entrevista com Gabriel)

---

## 1. Resumo executivo

ZapDesk é um aplicativo desktop para macOS com a experiência do WhatsApp (conversas, grupos, mídia, status) preparado para disparo em massa a listas de leads e controlável por IA através de um servidor MCP. Tudo roda na máquina do usuário: o motor de WhatsApp é um executável local leve que liga quando o app abre e desliga quando ele fecha. O objetivo é permitir que o Gabriel gere leads com IA, importe-os sem duplicar, dispare mensagens em massa e converse com quem responde — tudo num só lugar, sem servidor na nuvem e sem a Evolution API.

---

## 2. Contexto e motivação

### 2.1 Problema
Gerenciar prospecção por WhatsApp hoje exige juntar ferramentas: o WhatsApp oficial (sem disparo em massa), servidores como a Evolution API (pesados, feitos para ficar ligados o tempo todo, com Docker/Postgres/Redis) e planilhas de leads sem deduplicação. Não há como a IA que gera os leads alimentar diretamente o disparo.

### 2.2 Quem é afetado
O próprio Gabriel, operando prospecção de leads frios com um ou mais números de WhatsApp.

### 2.3 O que existe hoje
Evolution API foi considerada e descartada (ver §14). Nenhuma ferramenta local atual combina chat completo + disparo + MCP.

### 2.4 Resultado esperado
Um app leve que: (a) substitui o WhatsApp no dia a dia; (b) dispara em massa com relatório; (c) recebe leads da IA via MCP, avisando quais já existem; (d) não deixa nenhum processo rodando quando fechado (exceto enquanto um disparo em andamento termina).

---

## 3. Escopo

### 3.1 Dentro do escopo (MVP)
- **Contas:** várias contas de WhatsApp conectadas por QR code, alternáveis na interface; sessão persistida (não pede QR a cada abertura).
- **Chat estilo WhatsApp:** lista de conversas com não lidas; ler e enviar texto; grupos; busca de mensagens; lista de contatos; iniciar conversa com número novo.
- **Mídia:** ver e enviar imagem, vídeo, áudio, documento; tocar áudio; gravar e enviar áudio pelo microfone.
- **Interações:** responder citando, reagir, apagar para todos, editar mensagem.
- **Status/stories:** ver status dos contatos. **Figurinhas:** ver e enviar.
- **Etiquetas e notas** em contatos (estilo CRM).
- **Templates:** salvar mensagens (texto + anexo opcional) e inseri-las rapidamente numa conversa (ex.: atalho `/` no campo de mensagem).
- **Histórico:** importar o histórico que o WhatsApp envia na conexão; daí em diante tudo salvo localmente.
- **Disparo em massa:**
  - Lista por: importar CSV/planilha (XLSX), colar números (um por linha), escolher contatos/etiquetas, ou via MCP.
  - Mensagem: texto com variáveis (`{nome}` e demais colunas da lista), anexo opcional com legenda.
  - Agendamento: início em data/hora e janela de envio (ex.: só entre 9h e 18h).
  - Ritmo: **configurado pelo usuário em cada disparo** (intervalo mínimo/máximo aleatório, limite por hora, limite por dia, pausa a cada N mensagens). Sem preset imposto.
  - Relatório: por destinatário — pendente, enviado, entregue, lido, falhou (com motivo), respondeu; exportar CSV.
  - Pausar, retomar e cancelar disparo.
- **Leads:** base local de leads com deduplicação por telefone normalizado (E.164).
- **MCP (Claude Code e Claude Desktop):** tudo que um usuário pode fazer no app — importar leads (com relatório de duplicados), ler conversas/mensagens, enviar mensagem individual, criar/iniciar/pausar/cancelar disparos (**começa direto, sem confirmação**), ver relatórios, gerenciar templates, etiquetas e contatos, listar contas.
- **Ciclo de vida:** abrir o app liga o motor; fechar o app desliga tudo. Se houver disparo em andamento, fechar a janela mantém o app na barra de menu até o disparo terminar; ao terminar, mostra notificação com o resumo e encerra tudo sozinho. Se o MCP for chamado com o app fechado, o MCP abre o app.

### 3.2 Fora do escopo (não fazer agora)
- Chamadas de voz e vídeo (tecnicamente impossível com whatsmeow).
- Windows e Linux (só macOS).
- Lista de opt-out / descadastro (decisão explícita do usuário — ver §12).
- Rodízio de números ou variação automática de texto para evitar detecção de spam.
- API oficial (WhatsApp Business Cloud API).
- Sincronização em nuvem / multiusuário.
- Publicar status próprio, criar/administrar grupos e comunidades, canais (a confirmar para v2).

### 3.3 Roadmap futuro (v2+)
- Windows/Linux.
- Chatbot/respostas automáticas.
- Funil/CRM mais completo sobre as etiquetas.
- Assinatura e notarização Apple para distribuição fora da máquina do Gabriel.

---

## 4. Personas e usuários

| Persona | Papel/Permissão | Contexto de uso |
|---------|-----------------|-----------------|
| Gabriel (operador) | Único usuário, acesso total | Usa o app no Mac para conversar e disparar |
| IA (Claude Code / Claude Desktop) | Acesso total via MCP, mesmo poder do usuário | Gera leads, importa, dispara, lê respostas |

Não há login no app: é local e de um usuário só.

---

## 5. Fluxos UX

### 5.1 Fluxo principal (happy path)
1. Usuário abre o ZapDesk. O app sobe o motor local (< 2 s) e mostra a tela principal.
2. Primeira vez: tela "Conectar conta" com QR code. Usuário escaneia no celular (Aparelhos conectados). O app mostra "Sincronizando histórico…" e depois as conversas.
3. Usuário conversa normalmente (texto, mídia, áudio, reações, citações, figurinhas).
4. Usuário vai em **Disparos → Novo disparo**: escolhe a conta, importa CSV (mapeia colunas: telefone obrigatório, nome e outras opcionais), vê quantos são válidos, novos e duplicados.
5. Escreve a mensagem com variáveis (ou escolhe um template), anexa arquivo opcional, vê pré-visualização com uma linha real da lista.
6. Configura ritmo (intervalo, limites, janela) e agendamento; clica **Iniciar**.
7. Acompanha o progresso em tempo real; respostas aparecem nas conversas e marcam o destinatário como "respondeu".
8. Ao fim, exporta o relatório.
9. Fecha o app: motor e processos são encerrados.

### 5.2 Fluxos alternativos
- **Via IA:** Claude chama `importar_leads` com uma lista → recebe quais foram inseridos e quais já existiam (com data de importação) → chama `criar_disparo` → o disparo começa imediatamente.
- **MCP com app fechado:** a ferramenta MCP inicia o app, espera o motor ficar pronto e executa.
- **Fechar com disparo rodando:** janela some, ícone na barra de menu mostra progresso; ao terminar, notificação + encerramento total.
- **Várias contas:** seletor de conta no topo da lista de conversas; cada conta tem suas conversas; disparo sempre ligado a uma conta.
- **Sessão expirada:** conta aparece como "Desconectada" com botão "Reconectar" (novo QR).

### 5.3 Telas e componentes
- Barra lateral: contas, Conversas, Contatos, Status, Disparos, Leads, Templates, Etiquetas, Ajustes.
- Conversas: lista (busca, filtro por etiqueta, não lidas) + painel de chat (bolhas, mídia, citação, reações, gravação de áudio, seletor de figurinha, atalho `/` para templates) + painel lateral do contato (etiquetas, notas, lead de origem).
- Contatos, Status (visualizador), Leads (tabela com busca, origem, data, último disparo), Templates (CRUD), Etiquetas (CRUD com cor).
- Disparos: lista; assistente de novo disparo (lista → mensagem → ritmo/agendamento → revisão); detalhe com progresso e relatório.
- Conectar conta (QR). Ajustes (contas, pasta de dados, instruções de instalação do MCP).
- Ícone de barra de menu (apenas enquanto disparo mantém o app vivo).
- Sem Figma: visual inspirado no WhatsApp Desktop, tema claro/escuro seguindo o sistema.

### 5.4 Estados de cada tela
- **Loading:** esqueleto na lista de conversas; "Ligando o WhatsApp…" enquanto o motor sobe; "Sincronizando histórico…" após QR.
- **Vazio:** "Nenhuma conta conectada — Conectar conta"; "Nenhum disparo ainda — Novo disparo"; "Nenhum lead — importe um CSV ou use o MCP".
- **Erro:** motor não subiu → tela com motivo e "Tentar de novo"; conta desconectada → faixa amarela com "Reconectar"; mensagem com falha → ícone de erro na bolha com "Reenviar".
- **Sucesso:** estados normais.
- **Parcial/em progresso:** disparo com barra de progresso, contadores por status, próximo envio previsto, estado (agendado, enviando, fora da janela, pausado, concluído, cancelado).

### 5.5 Copy crítico
Interface em **português do Brasil**. Tom direto. Exemplos: "Ligando o WhatsApp…", "Escaneie o QR code no seu celular em Aparelhos conectados", "32 números já estavam na sua base", "Disparo concluído: 480 enviados, 12 falharam", "Há um disparo em andamento. O ZapDesk vai continuar na barra de menu até terminar."

### 5.6 Plataformas e acessibilidade
- Plataformas: macOS (Apple Silicon; Intel desejável).
- Acessibilidade: navegação por teclado nas ações principais (enviar, buscar, trocar conversa), rótulos em botões de ícone.
- i18n: só pt-BR.

---

## 6. Regras de negócio e modelo de dados

Armazenamento local em SQLite, na pasta de dados do app (`~/Library/Application Support/ZapDesk/`). A sessão do whatsmeow fica em banco próprio por conta.

### 6.1 Entidades

#### `Conta`
| Campo | Tipo | Obrigatório | Default | Validação |
|-------|------|-------------|---------|-----------|
| id | texto | sim | gerado | — |
| jid | texto | após conectar | — | JID do WhatsApp |
| nome | texto | sim | pushname | 1–60 chars |
| estado | enum | sim | `desconectada` | conectando, conectada, desconectada, banida |
| criada_em | data | sim | agora | — |

#### `Contato` / `Conversa` / `Mensagem`
Espelham o WhatsApp: conversa por JID (individual ou grupo) e por conta; mensagem com id do WhatsApp, remetente, tipo (texto, imagem, vídeo, áudio, documento, figurinha, reação, sistema), texto, caminho local da mídia, citação, reações, editada/apagada, estado (pendente, enviada, entregue, lida, falhou), data. Contato com notas (texto livre) e etiquetas (N:N).

#### `Etiqueta`
| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| nome | texto | sim | único, 1–30 chars |
| cor | texto | sim | hex |

#### `Lead`
| Campo | Tipo | Obrigatório | Default | Validação |
|-------|------|-------------|---------|-----------|
| id | texto | sim | gerado | — |
| telefone | texto | sim | — | normalizado E.164, **único** |
| nome | texto | não | — | — |
| campos | JSON | não | `{}` | colunas extras usadas como variáveis |
| origem | texto | sim | — | `csv`, `colado`, `contatos`, `mcp` |
| tem_whatsapp | bool/nulo | não | nulo | preenchido quando verificado |
| importado_em | data | sim | agora | — |

#### `Template`
id, nome (único), texto (com variáveis), anexo opcional (caminho local), criado/atualizado.

#### `Disparo`
| Campo | Tipo | Obrigatório | Validação |
|-------|------|-------------|-----------|
| id | texto | sim | — |
| conta_id | texto | sim | conta existente |
| nome | texto | sim | — |
| mensagem | texto | sim | não vazio |
| anexo | texto | não | arquivo existente |
| intervalo_min_s / intervalo_max_s | inteiro | sim | min ≥ 1, max ≥ min |
| limite_por_hora / limite_por_dia | inteiro | não | > 0 |
| pausa_a_cada / pausa_duracao_s | inteiro | não | > 0 |
| inicio_em | data | não | agora se vazio |
| janela_inicio / janela_fim | hora | não | ambos ou nenhum |
| estado | enum | sim | ver 6.4 |
| origem | texto | sim | `app` ou `mcp` |

#### `Destinatario` (do disparo)
disparo_id, lead_id, telefone, variáveis resolvidas, estado (pendente, enviando, enviado, entregue, lido, falhou, respondeu), motivo da falha, id da mensagem enviada, datas de cada transição.

**Relações:** Conta 1:N Conversa, Conversa 1:N Mensagem, Contato N:N Etiqueta, Disparo 1:N Destinatario, Lead 1:N Destinatario.

### 6.2 Regras de validação
- Telefone normalizado para E.164 com DDI padrão +55 quando ausente; números inválidos são rejeitados e listados no relatório de importação.
- Deduplicação de lead **somente por telefone normalizado**. Importar um telefone existente não cria novo lead; a resposta informa "já existia" e a data de importação original. Campos novos não sobrescrevem os existentes (a confirmar — §13).
- Duplicados dentro do mesmo lote contam uma vez.
- Variável na mensagem sem valor para um destinatário: bloqueia o início e lista quais linhas estão incompletas (ou o usuário escolhe um valor padrão).
- Anexo acima do limite do WhatsApp é recusado ao anexar.

### 6.3 Regras de autorização
_Não se aplica_ — usuário único local. O MCP tem o mesmo poder do usuário. A API local do motor só aceita pedidos de `127.0.0.1` com o token de sessão gerado a cada abertura.

### 6.4 Estados e transições
Disparo: `rascunho → agendado → enviando ⇄ fora_da_janela → concluido`; `enviando/agendado/fora_da_janela → pausado → enviando`; qualquer estado não final `→ cancelado`. Se o app fechar com disparo ativo, o app não fecha (fica na barra de menu); se o processo morrer mesmo assim, o disparo volta como `pausado` na próxima abertura e retoma sem reenviar destinatários já enviados.

Destinatário: `pendente → enviando → enviado → entregue → lido`; qualquer um `→ falhou`; `enviado/entregue/lido → respondeu` quando chega mensagem do número após o envio.

### 6.5 Cálculos e derivações
- Próximo envio = agora + aleatório uniforme entre intervalo_min e intervalo_max, respeitando limites por hora/dia, pausas e janela.
- Estimativa de término exibida antes de iniciar e durante o disparo.

### 6.6 Limites e quotas
Definidos pelo usuário em cada disparo (sem preset). O app exibe, ao configurar, um aviso sobre o risco de ban em ritmos agressivos, mas não bloqueia.

### 6.7 Compliance e privacidade
Todos os dados ficam locais. O disparo a leads frios sem consentimento e sem opt-out tem risco sob a LGPD e viola os termos do WhatsApp — risco assumido pelo usuário (§12). O app não envia dados a terceiros.

---

## 7. Aspectos técnicos

### 7.1 Stack e padrões do projeto
Monorepo novo em `~/projetos/zapdesk`, com repositório privado no GitHub.
- `motor/` — **Go** com **whatsmeow**: um executável; multi-conta (um cliente whatsmeow por conta); SQLite (sessões + dados do app); API HTTP + WebSocket (eventos) em `127.0.0.1` porta aleatória com token; agendador de disparos. **É a fonte única da verdade** — interface e MCP falam só com ele.
- `app/` — **Electron** + React + TypeScript + Vite. O processo principal inicia o motor como processo filho, passa porta/token, grava `runtime.json` (porta, token, pid) na pasta de dados, encerra o motor ao sair, controla o ícone da barra de menu.
- `mcp/` — servidor MCP em TypeScript (`@modelcontextprotocol/sdk`, stdio), empacotado junto do app. Lê `runtime.json`; se o app não estiver rodando, abre o app e espera o motor ficar pronto.
- Nomes, comentários e documentação em português.

### 7.2 Bibliotecas
**Usar:** `go.mau.fi/whatsmeow`; driver SQLite (preferir puro Go, ex. `modernc.org/sqlite`, para build simples); `nyaruka/phonenumbers` (ou equivalente) para E.164; Electron + electron-builder; React; `@modelcontextprotocol/sdk`; biblioteca de leitura de CSV/XLSX.
**Evitar:** Evolution API (pesada, feita para servidor sempre ligado); Docker em qualquer parte do runtime; Baileys (motor duplicado).

### 7.3 Integrações externas
| Sistema | Propósito | Tipo | Notas |
|---------|-----------|------|-------|
| WhatsApp (protocolo multi-device) | Mensagens | whatsmeow (não oficial) | Pode quebrar em atualizações do WhatsApp |
| Claude Code / Claude Desktop | Controle por IA | MCP stdio | Instruções de configuração em Ajustes |

### 7.4 Autenticação e autorização
Conta WhatsApp por QR code (Aparelhos conectados). API do motor protegida por token aleatório por sessão + bind em 127.0.0.1. `runtime.json` com permissão 0600.

### 7.5 Performance e escala
- Motor ocioso: alvo < 80 MB RAM; subir em < 2 s.
- Lista de conversas fluida com dezenas de milhares de mensagens (paginação + índices por conversa/data).
- Disparo: até dezenas de milhares de destinatários por disparo; o limite prático é o ritmo configurado.
- Índices: lead.telefone (único), mensagem(conversa, data), destinatario(disparo, estado).

### 7.6 Background jobs e async
Agendador de disparos dentro do motor (goroutine por disparo ativo, persistindo cada transição). Download de mídia sob demanda com cache local.

### 7.7 Observabilidade
Logs estruturados do motor em arquivo rotativo na pasta de dados (sem conteúdo de mensagens por padrão). Tela de Ajustes mostra versão e caminho dos logs. Sem telemetria externa.

### 7.8 Testes
- Unitários (Go): normalização de telefone, deduplicação, cálculo de agendamento/janela/limites, resolução de variáveis, máquina de estados do disparo.
- Integração: API HTTP do motor com cliente WhatsApp falso (interface abstrata sobre whatsmeow).
- MCP: testes das ferramentas contra motor com cliente falso.
- E2E manual: conectar conta real, conversar, disparo pequeno para números próprios.

### 7.9 Feature flag e rollout
_Não se aplica_ — app de uso pessoal.

---

## 8. Edge cases e tratamento de erros

| Cenário | Comportamento esperado | Mensagem ao usuário |
|---------|------------------------|---------------------|
| Motor não sobe (porta, binário ausente) | Tela de erro com detalhe e "Tentar de novo" | "Não consegui ligar o WhatsApp." |
| Motor cai com app aberto | App reinicia o motor uma vez; se cair de novo, tela de erro; disparos ficam `pausado` | "O WhatsApp parou. Religando…" |
| App fechado à força com disparo ativo | Na próxima abertura o disparo aparece `pausado`; retomar não reenvia os já enviados | "Disparo pausado porque o app foi fechado." |
| Sessão desconectada/expirada | Conta `desconectada`, disparos dessa conta pausam | "Conta desconectada. Reconecte para continuar." |
| Conta banida | Conta `banida`, disparos param | "O WhatsApp bloqueou este número." |
| Número sem WhatsApp no disparo | Destinatário `falhou` com motivo; disparo segue | "Sem WhatsApp" no relatório |
| Várias falhas seguidas no disparo | _a confirmar (§13)_ — sugestão: pausar após N falhas seguidas | "Disparo pausado após muitas falhas seguidas." |
| CSV sem coluna de telefone | Bloqueia import, pede mapeamento | "Escolha qual coluna tem o telefone." |
| Telefone inválido | Linha rejeitada e listada | "12 números inválidos" |
| Lead duplicado (app ou MCP) | Não cria; retorna "já existia" + data | "32 já estavam na base" |
| Variável sem valor | Bloqueia início e lista linhas | "5 contatos sem {nome}" |
| Fora da janela de envio | Estado `fora_da_janela`; retoma sozinho | "Aguardando janela (9h–18h)" |
| Mac dorme durante disparo | Ao acordar, recalcula agenda e segue | — |
| Sem internet | whatsmeow reconecta; envios aguardam; nada é marcado falho por queda curta | "Sem conexão. Tentando reconectar…" |
| MCP chamado com app fechado | MCP abre o app e espera até ~30 s pelo motor | Erro claro se não subir |
| Dois disparos na mesma conta | _a confirmar (§13)_ | — |
| Mídia não baixa | Bolha com "Tentar baixar de novo" | "Não foi possível baixar." |
| Edição/apagar fora do prazo do WhatsApp | Botão indisponível | — |

---

## 9. Plano de implementação

### 9.1 Etapas sugeridas (em ordem)
1. Estrutura do monorepo, constituição Spec Kit, CI básico.
2. Motor Go: conexão multi-conta com QR, persistência de sessão, API HTTP/WebSocket com token, ciclo de vida.
3. Motor: mensagens (receber/enviar texto e mídia, histórico, recibos, reações, citação, editar/apagar, figurinhas, status, grupos, contatos).
4. Motor: leads (normalização, deduplicação, importação), templates, etiquetas/notas.
5. Motor: disparos (agendador, ritmo, janela, limites, estados, relatório, retomada).
6. App Electron: iniciar/encerrar motor, `runtime.json`, barra de menu, telas de chat.
7. App: telas de disparo, leads, templates, etiquetas, ajustes.
8. MCP: ferramentas cobrindo todas as ações; abrir o app quando fechado.
9. Empacotamento macOS (.app/.dmg) incluindo motor e MCP; instruções de configuração do MCP.
10. Testes e verificação E2E com conta real.

### 9.2 Arquivos provavelmente afetados
Repositório novo: `motor/`, `app/`, `mcp/`, `docs/`, `specs/`, `.specify/`, `.github/workflows/`.

### 9.3 Dependências de outras features ou times
Go 1.27 instalado (Homebrew). Um número de WhatsApp de teste para E2E.

### 9.4 Estimativa de esforço
_Não solicitada._

---

## 10. Critérios de aceite

- [ ] Abrir o app sobe o motor em < 2 s; fechar (sem disparo ativo) não deixa nenhum processo do ZapDesk rodando (`ps` limpo).
- [ ] Conectar duas contas por QR; reabrir o app não pede QR de novo; trocar entre contas mostra conversas separadas.
- [ ] Enviar e receber texto, imagem, vídeo, áudio (inclusive gravado), documento e figurinha; responder citando, reagir, editar e apagar funcionam e aparecem no celular.
- [ ] Grupos aparecem e aceitam mensagens; status dos contatos é visível; busca encontra mensagens antigas.
- [ ] Etiquetas e notas em contatos; filtro por etiqueta na lista de conversas.
- [ ] Template inserido numa conversa via `/`.
- [ ] Importar CSV com 1.000 linhas contendo duplicados e inválidos: relatório correto de novos, já existentes e inválidos.
- [ ] Disparo com variáveis e anexo respeita intervalo aleatório, limites, janela e agendamento; relatório mostra enviado/entregue/lido/falhou/respondeu e exporta CSV.
- [ ] Pausar, retomar e cancelar disparo; matar o app no meio e reabrir retoma sem reenviar.
- [ ] Fechar a janela com disparo ativo mantém o ícone na barra de menu; ao concluir, notificação e encerramento total.
- [ ] Pelo Claude Code e pelo Claude Desktop: importar leads (retorna duplicados), criar disparo que começa na hora, ler respostas, enviar mensagem, gerenciar templates/etiquetas.
- [ ] Chamar o MCP com o app fechado abre o app e conclui a ação.
- [ ] Motor ocioso usa < 80 MB de RAM.

---

## 11. Métricas de sucesso e observabilidade pós-launch

### 11.1 KPIs de produto
Uso pessoal: disparos concluídos sem intervenção; taxa de resposta por disparo; tempo gasto importando leads (manual vs MCP).

### 11.2 Eventos a instrumentar
_Não se aplica_ — sem telemetria externa; o relatório de disparos e os logs locais cobrem.

### 11.3 Plano de rollback
Versões do app em `.dmg`; manter a anterior. Migrações do SQLite versionadas e com backup automático do banco antes de migrar.

---

## 12. Riscos e mitigações

| Risco | Probabilidade | Impacto | Mitigação |
|-------|---------------|---------|-----------|
| Banimento do número (leads frios, sem opt-out, IA dispara sem confirmação) | alta | alto | Ritmo configurável, estado `banida` detectado e disparos parados; usar número dedicado; aviso ao configurar ritmo agressivo |
| LGPD — mensagens sem consentimento | média | alto | Risco assumido pelo usuário; dados só locais |
| whatsmeow quebrar após mudança do WhatsApp | média | alto | Isolar whatsmeow atrás de interface; atualizar dependência |
| IA disparar para lista errada via MCP (sem confirmação) | média | alto | Relatório de importação detalhado; ferramenta de pausar/cancelar via MCP e no app |
| Mac dormir/fechar no meio do disparo | alta | médio | Retomada idempotente por destinatário |
| App não assinado bloqueado pelo Gatekeeper | alta | baixo | Uso local; instruções de "Abrir mesmo assim"; notarização no v2 |

---

## 13. Questões em aberto

- [ ] Importar lead existente com campos novos: atualizar campos vazios ou nunca tocar? (proposta: preencher só campos vazios)
- [ ] Pausar disparo automaticamente após N falhas seguidas? Qual N? (proposta: pausar após 10 falhas seguidas, configurável)
- [ ] Permitir dois disparos simultâneos na mesma conta? (proposta: não — o segundo fica na fila)
- [ ] Verificar em lote se o número tem WhatsApp antes do disparo (não foi escolhido como critério de duplicado, mas o motor suporta)?
- [ ] Nome definitivo do app (ZapDesk é provisório).

---

## 14. Decisões registradas (ADR-style)

- **Decisão:** whatsmeow (Go) em vez de Evolution API.
  **Razão:** precisa ser leve e ligar/desligar junto do app; Evolution exige servidor sempre ligado com Docker/Postgres/Redis.
- **Decisão:** Electron (escolha do usuário), apesar da recomendação de Tauri.
  **Razão:** preferência do usuário.
- **Decisão:** motor Go é a fonte única da verdade (dados + agendador); UI e MCP são clientes.
  **Razão:** app e IA veem o mesmo estado; disparo continua independente da janela.
- **Decisão:** disparo via MCP começa sem confirmação.
  **Razão:** escolha do usuário, priorizando automação.
- **Decisão:** sem lista de opt-out; ritmo sem preset.
  **Razão:** escolha do usuário.
- **Decisão:** fechar com disparo ativo mantém o app na barra de menu até terminar e então encerra tudo.
  **Razão:** equilibrar "desligar ao fechar" com não interromper disparos.
- **Decisão:** MCP abre o app quando chamado com ele fechado.
  **Razão:** automação pela IA sem passo manual.

---

## 15. Referências

- whatsmeow: https://github.com/tulir/whatsmeow
- Model Context Protocol: https://modelcontextprotocol.io
- Entrevista de 2026-09-27 (sessão Claude Code).
