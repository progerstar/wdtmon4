import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import axios from 'axios';
import TabConnect from '../src/TabConnect';
import { afterEach, vi } from 'vitest';

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

test('clear uses the dedicated endpoint before resetting cloud state', async () => {
  const setSettings = vi.fn();
  const confirm = vi.fn(() => true);
  vi.stubGlobal('confirm', confirm);
  vi.spyOn(axios, 'post').mockResolvedValueOnce({data: {}});

  render(<TabConnect settings={{
    ConUID: "utx1_token",
    ConWriteToken: "utx1_token",
    ConReadToken: "",
    ConDev: "device-1",
  }} setSettings={setSettings} />);

  fireEvent.click(screen.getByRole('button', {name: 'Clear tokens'}));

  expect(confirm).toHaveBeenCalledWith('Clear tokens confirmation');
  expect(axios.post).toHaveBeenCalledWith(
    '/con/clear',
    undefined,
    expect.objectContaining({timeout: 5000, signal: expect.any(AbortSignal)}),
  );
  await waitFor(() => expect(setSettings).toHaveBeenCalledWith({
    ConEn: false,
    ConUID: "", ConReadToken: "", ConWriteToken: "", ConDashboardURL: "",
    ConConfigured: false,
  }));
});

test('tokens can be revealed without relying on hover', () => {
  const token = `utx1_${'a'.repeat(86)}`;

  render(<TabConnect settings={{ConWriteToken: token}} setSettings={vi.fn()} />);

  expect(screen.queryByText(token)).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', {name: 'Show tokens'}));
  expect(screen.getByText(token)).toBeInTheDocument();
});

test('does not render a dashboard link with an unsafe protocol', () => {
  render(<TabConnect settings={{
    ConConfigured: true,
    ConDashboardURL: 'javascript:alert(1)',
    ConDev: 'device-1',
  }} setSettings={vi.fn()} />);

  expect(screen.queryByRole('link', {name: 'Dashboard'})).not.toBeInTheDocument();
  expect(screen.getByText('Write token is configured.')).toBeInTheDocument();
});

test('keeps cloud state when the clear request fails', async () => {
  vi.stubGlobal('confirm', vi.fn(() => true));
  vi.spyOn(axios, 'post').mockRejectedValueOnce(new Error('offline'));
  const setSettings = vi.fn();

  render(<TabConnect settings={{ConConfigured: true, ConDev: 'device-1'}} setSettings={setSettings} />);
  fireEvent.click(screen.getByRole('button', {name: 'Clear tokens'}));

  expect(await screen.findByRole('alert')).toHaveTextContent('Failed to clear tokens');
  expect(setSettings).not.toHaveBeenCalled();
});
