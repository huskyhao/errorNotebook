import { fireEvent, render, screen } from '@testing-library/react';
import TagEditor from './TagEditor';

test('does not show tag candidates when the tag field is focused', () => {
  render(
    <TagEditor
      questionTags={[]}
      allTags={[{ id: 1, name: '同步互斥' }]}
      onTagsChange={jest.fn()}
    />,
  );

  fireEvent.focus(screen.getByPlaceholderText('添加知识点...'));

  expect(document.querySelector('.tag-suggestions')).toBeNull();
  expect(screen.queryByText('同步互斥')).not.toBeInTheDocument();
});
