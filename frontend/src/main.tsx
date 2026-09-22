import { createRoot } from 'react-dom/client';
import App from './app/App.tsx';
import './index.css';
import { setBaseConfigSnapshot } from './lib/branding';

// prefetchBaseConfig 在应用挂载前拉取品牌配置，使启动屏首帧即为真实品牌；
// 超时 500ms 兜底，失败时使用缓存或默认品牌，不阻塞过久
async function prefetchBaseConfig() {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 500);
  try {
    const response = await fetch('/api/public/base-config', { signal: controller.signal });
    if (response.ok) {
      setBaseConfigSnapshot(await response.json());
      return;
    }
    console.warn('品牌配置预取失败，使用当前品牌', response.status);
  } catch (error) {
    console.warn('品牌配置预取失败，使用当前品牌', error);
  } finally {
    clearTimeout(timer);
  }
}

await prefetchBaseConfig();

createRoot(document.getElementById('root')!).render(<App />);
