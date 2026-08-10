import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import axios from 'axios';
import { beforeEach, expect, test, vi } from 'vitest';
import App from '../src/App';

vi.mock('axios', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

vi.mock('../src/TabMain', () => ({
  default: ({settings, setSettings}) => (
    <div>
      <output data-testid="device-id">{settings.ConDev}</output>
      <button type="button" onClick={() => setSettings({ConDev: 'invalid device'})}>
        Set invalid device ID
      </button>
      <button type="button" onClick={() => setSettings({
        ConDev: 'device-2',
        ConReadToken: `utx1_${'r'.repeat(86)}`,
        ConDashboardURL: 'https://cloud.unitx.pro/dashboard',
        ConWriteToken: `utx1_${'w'.repeat(86)}`,
        ConUID: `utx1_${'w'.repeat(86)}`,
        ConConfigured: true,
      })}>
        Set cloud session
      </button>
    </div>
  ),
}));

vi.mock('../src/TabSettings', () => ({default: () => null}));
vi.mock('../src/TabConnect', () => ({default: () => <div>Cloud connected</div>}));
vi.mock('../src/Login', () => ({default: () => <div>Cloud login</div>}));

const loadedSettings = {
  ConEn: true,
  ConConfigured: true,
  ConDev: 'device-1',
  Net: '',
  NetEn: false,
  Proc: '',
  ProcEn: false,
  Diode: true,
  Pause: false,
};

beforeEach(() => {
  axios.get.mockReset();
  axios.post.mockReset();
  axios.get.mockResolvedValue({data: loadedSettings});
});

test('shows the persisted cloud state even when the USB device is absent', async () => {
  render(<App />);

  const cloudToggle = await screen.findByRole('checkbox', {name: 'Cloud'});
  await waitFor(() => expect(cloudToggle).toBeChecked());
  expect(cloudToggle).toBeEnabled();
  expect(screen.getByText('Cloud connected')).toBeInTheDocument();
});

test('never includes cloud secrets in the generic settings request', async () => {
  axios.post.mockResolvedValue({data: {}});
  render(<App />);
  await waitFor(() => expect(screen.getByTestId('device-id')).toHaveTextContent('device-1'));

  fireEvent.click(screen.getByRole('button', {name: 'Set cloud session'}));

  await waitFor(() => expect(axios.post).toHaveBeenCalled());
  const [url, body] = axios.post.mock.calls[0];
  expect(url).toBe('/settings');
  expect(body).toMatchObject({ConDev: 'device-2', ConEn: true});
  for (const field of [
    'ConReadToken',
    'ConDashboardURL',
    'ConWriteToken',
    'ConUID',
    'ConConfigured',
  ]) {
    expect(body).not.toHaveProperty(field);
  }
});

test('rolls back the latest optimistic settings update after a final 4xx', async () => {
  let rejectSave;
  axios.post.mockImplementation(() => new Promise((_, reject) => { rejectSave = reject; }));
  render(<App />);
  await waitFor(() => expect(screen.getByTestId('device-id')).toHaveTextContent('device-1'));

  fireEvent.click(screen.getByRole('button', {name: 'Set invalid device ID'}));
  expect(screen.getByTestId('device-id')).toHaveTextContent('invalid device');

  await act(async () => {
    rejectSave({response: {status: 400}});
  });

  await waitFor(() => expect(screen.getByTestId('device-id')).toHaveTextContent('device-1'));
  expect(screen.getByRole('alert')).toHaveTextContent('Failed to save settings');
});
