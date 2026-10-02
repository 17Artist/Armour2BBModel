/* Conversion runs in a separate thread so the workspace stays responsive. */
'use strict';
let ready = false;
globalThis._wasmReady = () => { ready = true; postMessage({ type: 'ready' }); };
// Compatible with older builds that addressed their readiness callback via window.
globalThis.window = globalThis;

async function start() {
  try {
    importScripts('wasm_exec.js');
    const go = new Go();
    const response = await fetch('convert.wasm');
    if (!response.ok) throw new Error(`转换引擎下载失败（HTTP ${response.status}）`);
    let result;
    if (WebAssembly.instantiateStreaming) {
      try { result = await WebAssembly.instantiateStreaming(response.clone(), go.importObject); }
      catch { result = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject); }
    } else { result = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject); }
    go.run(result.instance).catch(error => postMessage({ type: 'engine-error', error: error.message }));
  } catch (error) { postMessage({ type: 'engine-error', error: error.message }); }
}

self.onmessage = event => {
  if (event.data.type !== 'convert') return;
  const { id, buffer, keepCoincidentFaces } = event.data;
  if (!ready) { postMessage({ type: 'error', id, error: '转换引擎尚未准备完成' }); return; }
  try {
    postMessage({ type: 'phase', id, message: '正在解析文件、匹配部件并合并体素…' });
    const started = performance.now();
    const result = globalThis.convertArmour(new Uint8Array(buffer), { keepCoincidentFaces: keepCoincidentFaces === true });
    if (result.error) throw new Error(result.error);
    if (!(result.data instanceof Uint8Array)) throw new Error('转换引擎没有返回模型数据');
    postMessage({ type: 'result', id, ...result, elapsedMs: performance.now() - started }, [result.data.buffer]);
  } catch (error) { postMessage({ type: 'error', id, error: error.message }); }
};
start();
