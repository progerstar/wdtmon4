import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import axios from 'axios';
import Login from '../src/Login';
import { afterEach, vi } from 'vitest';

const writeToken = `utx1_${'a'.repeat(86)}`;
const readToken = `utx1_${'b'.repeat(86)}`;

afterEach(() => {
  vi.restoreAllMocks();
});

test('does not accept an incomplete write token', () => {
  const setSettings = vi.fn();

  render(<Login setSettings={setSettings} />);
  fireEvent.change(screen.getByLabelText('Write token'), { target: { value: 'utx1_token' } });
  fireEvent.click(screen.getByRole('button', { name: 'Use existing token' }));

  expect(setSettings).not.toHaveBeenCalled();
  expect(screen.getByRole('alert')).toHaveTextContent('Invalid write token');
});

test('verifies a well-formed token with Cloud before saving it', async () => {
  const setSettings = vi.fn();
  vi.spyOn(axios, 'post').mockResolvedValueOnce({ data: { device_id: 'device-1' } });

  render(<Login setSettings={setSettings} />);
  fireEvent.change(screen.getByLabelText('Write token'), { target: { value: writeToken } });
  fireEvent.click(screen.getByRole('button', { name: 'Use existing token' }));

  expect(axios.post).toHaveBeenCalledWith(
    '/con/validate',
    { write_token: writeToken },
    expect.objectContaining({timeout: 12000, signal: expect.any(AbortSignal)}),
  );
  await waitFor(() => expect(setSettings).toHaveBeenCalledWith({
    ConUID: '',
    ConReadToken: '',
    ConWriteToken: '',
    ConDashboardURL: '',
    ConDev: 'device-1',
    ConConfigured: true,
  }));
});

test('does not save a token rejected by Cloud scope validation', async () => {
  const setSettings = vi.fn();
  const token = `utx1_${'b'.repeat(86)}`;
  vi.spyOn(axios, 'post').mockRejectedValueOnce({ response: { status: 403 } });

  render(<Login setSettings={setSettings} />);
  fireEvent.change(screen.getByLabelText('Write token'), { target: { value: token } });
  fireEvent.click(screen.getByRole('button', { name: 'Use existing token' }));

  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Invalid write token'));
  expect(setSettings).not.toHaveBeenCalled();
});

test('rejects a validation response without a valid device ID', async () => {
  const setSettings = vi.fn();
  vi.spyOn(axios, 'post').mockResolvedValueOnce({ data: { device_id: 'invalid device' } });

  render(<Login setSettings={setSettings} />);
  fireEvent.change(screen.getByLabelText('Write token'), { target: { value: writeToken } });
  fireEvent.click(screen.getByRole('button', { name: 'Use existing token' }));

  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Invalid cloud response'));
  expect(setSettings).not.toHaveBeenCalled();
});

test('keeps newly created tokens in memory but drops an unsafe dashboard URL', async () => {
  const setSettings = vi.fn();
  vi.spyOn(axios, 'post').mockResolvedValueOnce({
    data: {
      write_token: writeToken,
      read_token: readToken,
      device_id: 'device-2',
      dashboard_url: 'javascript:alert(1)',
    },
  });

  render(<Login setSettings={setSettings} />);
  fireEvent.click(screen.getByRole('button', { name: 'Get new tokens' }));

  expect(axios.post).toHaveBeenCalledWith(
    '/con/create',
    undefined,
    expect.objectContaining({timeout: 12000, signal: expect.any(AbortSignal)}),
  );
  await waitFor(() => expect(setSettings).toHaveBeenCalledWith({
    ConUID: writeToken,
    ConReadToken: readToken,
    ConWriteToken: writeToken,
    ConDashboardURL: '',
    ConDev: 'device-2',
    ConConfigured: true,
  }));
});
