// Workers do Monaco empacotados pelo Vite (sem CDN; CSP `worker-src 'self' blob:`).
// Caminhos novos do `exports` do monaco-editor 0.57 (research.md R12).
import EditorWorker from 'monaco-editor/editor/editor.worker?worker';
import JsonWorker from 'monaco-editor/language/json/json.worker?worker';
import TsWorker from 'monaco-editor/language/typescript/ts.worker?worker';

interface AmbienteMonaco {
  getWorker(idWorker: string, rotulo: string): Worker;
}

(self as unknown as { MonacoEnvironment: AmbienteMonaco }).MonacoEnvironment = {
  getWorker(_idWorker, rotulo) {
    if (rotulo === 'typescript' || rotulo === 'javascript') return new TsWorker();
    if (rotulo === 'json') return new JsonWorker();
    return new EditorWorker();
  },
};
