import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';

test('renders question workbench layout', () => {
  render(
    <MemoryRouter>
      <App />
    </MemoryRouter>,
  );
  expect(screen.getByText(/ErroNotebook/i)).toBeInTheDocument();
  expect(screen.getByRole('complementary', { name: '题目列表' })).toBeInTheDocument();
  expect(screen.getByRole('complementary', { name: '题目详情' })).toBeInTheDocument();
});
