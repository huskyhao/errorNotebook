import { analysisStatusLabel, ocrStatusLabel, requestJson } from './utils';

describe('async import contract', () => {
  it('labels queued and processing stages explicitly', () => {
    expect(ocrStatusLabel('queued')).toBe('OCR 排队中');
    expect(ocrStatusLabel('processing')).toBe('识别中');
    expect(analysisStatusLabel('queued')).toBe('解析排队');
    expect(analysisStatusLabel('processing')).toBe('解析中');
  });

  it('does not attach browser identity credentials to Go API requests', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({ data: { ok: true } }),
    } as Response);
    await requestJson<{ ok: boolean }>('/health');
    expect(fetchMock.mock.calls[0][1]).not.toHaveProperty('credentials');
    fetchMock.mockRestore();
  });
});
