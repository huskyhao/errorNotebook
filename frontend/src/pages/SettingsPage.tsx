import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import TopBar from '../components/TopBar';
import SideNavigation from '../components/SideNavigation';
import { SettingsIcon } from '../components/icons';
import { requestJson } from '../utils';

type ProviderState = { provider: string; model?: string | null; configured: boolean };
type ProviderSettings = { text: ProviderState; vision: ProviderState; source: 'server_env' | string };

const emptySettings: ProviderSettings = {
  text: { provider: 'unknown', model: null, configured: false },
  vision: { provider: 'unknown', model: null, configured: false },
  source: 'server_env',
};

function ProviderCard({ title, description, value }: { title: string; description: string; value: ProviderState }) {
  return (
    <section className="settings-card">
      <div className="settings-card-header"><h2>{title}</h2><p>{description}</p></div>
      <div className="settings-form-grid">
        <div className="form-field"><span className="form-label">Provider</span><div className="settings-readonly-value">{value.provider || '未配置'}</div></div>
        <div className="form-field"><span className="form-label">Model</span><div className="settings-readonly-value">{value.model || '未配置'}</div></div>
        <div className="form-field"><span className="form-label">API Key</span><div className="settings-readonly-value">{value.configured ? '已由服务端配置（不展示密钥）' : '未配置'}</div></div>
      </div>
    </section>
  );
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<ProviderSettings>(emptySettings);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    requestJson<ProviderSettings>('/settings/ai')
      .then((value) => { if (active) setSettings(value); })
      .catch((err) => { if (active) setError(err instanceof Error ? err.message : '读取 AI 配置失败'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  return (
    <div className="app-shell">
      <TopBar />
      <div className="settings-workspace">
        <SideNavigation />
        <main className="settings-page">
          <header className="settings-header">
            <div className="section-title"><SettingsIcon size={20} /><h1>设置</h1></div>
            <Link className="outline-button" to="/">返回工作台</Link>
          </header>
          <div className="settings-content">
            <div className="settings-intro"><p>AI Provider 配置由当前部署实例的服务端环境变量管理。浏览器不保存、不上传、不回显 API Key；页面只展示脱敏后的配置状态。</p></div>
            {loading ? <div className="empty-state">正在读取服务端配置...</div> : null}
            {error ? <div className="global-toast is-error">{error}</div> : null}
            {!loading ? <>
              <ProviderCard title="纯文本模型" description="用于无图片题目、普通追问和文本解析。" value={settings.text} />
              <ProviderCard title="多模态模型" description="用于图片 OCR、图形理解和图片题解析。" value={settings.vision} />
              <div className="settings-actions"><span className="save-feedback">配置来源：{settings.source === 'server_env' ? '服务端环境变量 / secrets' : settings.source}</span></div>
            </> : null}
          </div>
        </main>
      </div>
    </div>
  );
}
