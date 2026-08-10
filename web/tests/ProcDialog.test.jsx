import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import axios from 'axios';
import { afterEach, test, vi } from 'vitest';
import ProcDialog from '../src/ProcDialog';

afterEach(() => {
  vi.restoreAllMocks();
});

test('loads the process list and returns the selected process', async () => {
  const onChange = vi.fn();
  vi.spyOn(axios, 'get').mockResolvedValueOnce({
    data: [{name: 'very-long-background-process-name-that-needs-wrapping'}],
  });

  render(<ProcDialog proc="" onChange={onChange} disabled={false} />);
  const trigger = screen.getByRole('button', {name: 'Open process list'});
  fireEvent.click(trigger);

  const closeButton = screen.getByRole('button', {name: 'Close'});
  expect(closeButton).toHaveFocus();

  const processButton = await screen.findByRole('button', {
    name: 'very-long-background-process-name-that-needs-wrapping',
  });
  processButton.focus();
  fireEvent.keyDown(document, {key: 'Tab'});
  expect(closeButton).toHaveFocus();

  fireEvent.click(processButton);

  expect(onChange).toHaveBeenCalledWith('very-long-background-process-name-that-needs-wrapping');
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  expect(trigger).toHaveFocus();
});

test('ignores a stale process response after close and reopen', async () => {
  let resolveFirst;
  let resolveSecond;
  vi.spyOn(axios, 'get')
    .mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve; }))
    .mockImplementationOnce(() => new Promise((resolve) => { resolveSecond = resolve; }));

  render(<ProcDialog proc="" onChange={vi.fn()} disabled={false} />);
  const trigger = screen.getByRole('button', {name: 'Open process list'});
  fireEvent.click(trigger);
  fireEvent.click(screen.getByRole('button', {name: 'Close'}));
  fireEvent.click(trigger);

  await act(async () => {
    resolveSecond({data: [{name: 'new-process'}]});
  });
  expect(await screen.findByRole('button', {name: 'new-process'})).toBeInTheDocument();

  await act(async () => {
    resolveFirst({data: [{name: 'stale-process'}]});
  });
  await waitFor(() => expect(screen.queryByRole('button', {name: 'stale-process'})).not.toBeInTheDocument());
});
