import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import SettingsPage from './SettingsPage';

test('reads redacted provider status from Go and never renders an API key input', async () => {
  const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue({
    ok: true,
    json: async () => ({ data: {
      text: { provider: 'openai_compatible', model: 'text-model', configured: true },
      vision: { provider: 'not_configured', model: null, configured: false },
      source: 'server_env',
    } }),
  } as Response);

  render(<MemoryRouter><SettingsPage /></MemoryRouter>);

  await waitFor(() => expect(screen.getByText('已由服务端配置（不展示密钥）')).toBeInTheDocument());
  expect(screen.queryByRole('textbox', { name: /API Key/i })).not.toBeInTheDocument();
  expect(window.localStorage.getItem('erro-notebook:api-settings:v1')).toBeNull();
  expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/settings/ai'), expect.objectContaining({ headers: expect.any(Object) }));

  fetchMock.mockRestore();
});
