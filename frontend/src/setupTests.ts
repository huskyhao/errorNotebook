// jest-dom adds custom jest matchers for asserting on DOM nodes.
// allows you to do things like:
// expect(element).toHaveTextContent(/react/i)
// learn more: https://github.com/testing-library/jest-dom
import '@testing-library/jest-dom';
const mockReact = require('react');

// CRA's Jest runtime does not transform the ESM-only markdown toolchain used
// by the production bundle. The workbench tests exercise layout and actions,
// so keep MarkdownRenderer's test dependency lightweight and deterministic.
jest.mock('react-markdown', () => ({
  __esModule: true,
  default: ({ children }: { children: unknown }) => mockReact.createElement('div', null, children),
}));
jest.mock('remark-gfm', () => ({ __esModule: true, default: () => null }));
jest.mock('remark-math', () => ({ __esModule: true, default: () => null }));
jest.mock('rehype-katex', () => ({ __esModule: true, default: () => null }));
