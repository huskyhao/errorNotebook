import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import TopBar from '../components/TopBar';
import SideNavigation from '../components/SideNavigation';
import { SettingsIcon } from '../components/icons';

type ModelConfig = {
  baseUrl: string;
  apiKey: string;
  model: string;
};

type ApiSettings = {
  textModel: ModelConfig;
  visionModel: ModelConfig;
};

const STORAGE_KEY = 'erro-notebook:api-settings:v1';

const emptySettings: ApiSettings = {
  textModel: {
    baseUrl: '',
    apiKey: '',
    model: '',
  },
  visionModel: {
    baseUrl: '',
    apiKey: '',
    model: '',
  },
};

function readSettings(): ApiSettings {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return emptySettings;
    const parsed = JSON.parse(raw) as Partial<ApiSettings>;
    return {
      textModel: { ...emptySettings.textModel, ...parsed.textModel },
      visionModel: { ...emptySettings.visionModel, ...parsed.visionModel },
    };
  } catch {
    return emptySettings;
  }
}

function SettingsSection({
  title,
  description,
  value,
  onChange,
}: {
  title: string;
  description: string;
  value: ModelConfig;
  onChange: (value: ModelConfig) => void;
}) {
  return (
    <section className="settings-card">
      <div className="settings-card-header">
        <h2>{title}</h2>
        <p>{description}</p>
      </div>
      <div className="settings-form-grid">
        <label className="form-field">
          <span className="form-label">Base URL</span>
          <input
            className="form-input"
            value={value.baseUrl}
            placeholder="https://api.example.com/v1"
            onChange={(event) => onChange({ ...value, baseUrl: event.target.value })}
          />
        </label>
        <label className="form-field">
          <span className="form-label">API Key</span>
          <input
            className="form-input"
            type="password"
            value={value.apiKey}
            placeholder="输入 API Key"
            autoComplete="off"
            onChange={(event) => onChange({ ...value, apiKey: event.target.value })}
          />
        </label>
        <label className="form-field">
          <span className="form-label">Model</span>
          <input
            className="form-input"
            value={value.model}
            placeholder="例如 gpt-4.1-mini"
            onChange={(event) => onChange({ ...value, model: event.target.value })}
          />
        </label>
      </div>
    </section>
  );
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<ApiSettings>(emptySettings);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setSettings(readSettings());
  }, []);

  function handleSave() {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    setSaved(true);
    window.setTimeout(() => setSaved(false), 2200);
  }

  return (
    <div className="app-shell">
      <TopBar />
      <div className="settings-workspace">
        <SideNavigation />
        <main className="settings-page">
          <header className="settings-header">
            <div className="section-title">
              <SettingsIcon size={20} />
              <h1>设置</h1>
            </div>
            <Link className="outline-button" to="/">
              返回工作台
            </Link>
          </header>

          <div className="settings-content">
            <div className="settings-intro">
              <p>
                没图片的题目、普通追问和文本解析走纯文本模型；包含图片的题目、OCR/视觉理解和图片题解析走多模态模型。
              </p>
            </div>

            <SettingsSection
              title="纯文本模型"
              description="用于无图片题目、普通追问、文本解析等链路。"
              value={settings.textModel}
              onChange={(textModel) => {
                setSettings((prev) => ({ ...prev, textModel }));
                setSaved(false);
              }}
            />

            <SettingsSection
              title="多模态模型"
              description="用于包含图片的题目、OCR/视觉理解、图片题解析等链路。"
              value={settings.visionModel}
              onChange={(visionModel) => {
                setSettings((prev) => ({ ...prev, visionModel }));
                setSaved(false);
              }}
            />

            <div className="settings-actions">
              <button className="primary-button" type="button" onClick={handleSave}>
                保存设置
              </button>
              {saved ? <span className="save-feedback">已保存</span> : null}
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}
