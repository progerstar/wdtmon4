import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import axios from 'axios';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import TabMain from '../src/TabMain';

vi.mock('axios', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

const settings = {
  Net: '',
  NetEn: false,
  Proc: '',
  ProcEn: false,
  Diode: true,
  Pause: false,
};

describe('TabMain device controls', () => {
  beforeEach(() => {
    axios.get.mockReset();
    axios.post.mockReset();
    axios.get.mockImplementation((url) => {
      if (url === '/uptime') return Promise.resolve({data: 5});
      return Promise.reject(new Error(`unexpected URL: ${url}`));
    });
    axios.post.mockImplementation((url) => {
      switch (url) {
        case '/cmd/~I':
          return Promise.resolve({data: '~IWatchDog'});
        case '/cmd/~U':
          return Promise.resolve({data: '~A'});
        case '/cmd/~G':
          return Promise.resolve({data: '23.5'});
        case '/cmd/~T1':
          return Promise.reject(new Error('serial timeout'));
        default:
          return Promise.reject(new Error(`unexpected URL: ${url}`));
      }
    });
    vi.stubGlobal('confirm', vi.fn(() => true));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  test('shows a visible error when a reset command fails', async () => {
    const setSettings = vi.fn();
    const setPresentExt = vi.fn();

    render(
      <TabMain
        settings={settings}
        setSettings={setSettings}
        setPresentExt={setPresentExt}
      />,
    );

    const resetButton = screen.getByRole('button', {name: 'Reset'});
    await waitFor(() => expect(resetButton).toBeEnabled());

    fireEvent.click(resetButton);

    expect(await screen.findByRole('alert')).toHaveTextContent('Device command failed');
    expect(confirm).toHaveBeenCalledWith('Restart the connected PC now?');
    expect(axios.post).toHaveBeenCalledWith(
      '/cmd/~T1',
      undefined,
      expect.objectContaining({timeout: 3000, signal: expect.any(AbortSignal)}),
    );
  });

  test('describes network monitoring as a TCP endpoint', async () => {
    render(
      <TabMain
        settings={settings}
        setSettings={vi.fn()}
        setPresentExt={vi.fn()}
      />,
    );

    const target = await screen.findByRole('textbox', {
      name: 'TCP endpoint monitoring',
    });
    expect(target).toHaveAttribute('placeholder', 'host, host:port, https://host…');
    expect(target).toHaveAttribute('title', 'Host, host:port or URL. A plain host uses port 80.');
  });

  test('does not send a destructive command when confirmation is cancelled', async () => {
    confirm.mockReturnValue(false);

    render(
      <TabMain settings={settings} setSettings={vi.fn()} setPresentExt={vi.fn()} />,
    );

    const shutdownButton = screen.getByRole('button', {name: 'Shutdown'});
    await waitFor(() => expect(shutdownButton).toBeEnabled());
    fireEvent.click(shutdownButton);

    expect(confirm).toHaveBeenCalledWith('Shut down the connected PC now?');
    expect(axios.post.mock.calls.some(([url]) => url === '/cmd/~T3')).toBe(false);
  });

  test('rejects a malformed destructive command acknowledgement', async () => {
    axios.post.mockImplementation((url) => {
      switch (url) {
        case '/cmd/~I': return Promise.resolve({data: '~IWatchDog'});
        case '/cmd/~U': return Promise.resolve({data: '~A'});
        case '/cmd/~G': return Promise.resolve({data: '23.5'});
        case '/cmd/~T2': return Promise.resolve({data: 'Blocked'});
        default: return Promise.reject(new Error(`unexpected URL: ${url}`));
      }
    });

    render(<TabMain settings={settings} setSettings={vi.fn()} setPresentExt={vi.fn()} />);

    const powerButton = screen.getByRole('button', {name: 'Power'});
    await waitFor(() => expect(powerButton).toBeEnabled());
    fireEvent.click(powerButton);

    expect(await screen.findByRole('alert')).toHaveTextContent('Device command failed');
    expect(axios.post).toHaveBeenCalledWith(
      '/cmd/~T2',
      undefined,
      expect.objectContaining({timeout: 3000, signal: expect.any(AbortSignal)}),
    );
  });

  const mockDevice = (responses) => {
    axios.post.mockImplementation((url) => {
      switch (url) {
        case '/cmd/~I': return Promise.resolve({data: '~IWatchDog'});
        case '/cmd/~U': return Promise.resolve({data: '~A'});
        case '/cmd/~G': return Promise.resolve({data: '23.5'});
        default:
          if (Object.hasOwn(responses, url)) return Promise.resolve(responses[url]);
          return Promise.reject(new Error(`unexpected URL: ${url}`));
      }
    });
  };

  test('accepts an unacknowledged power command', async () => {
    mockDevice({'/cmd/~T1': {status: 202, data: 'Accepted'}});

    render(<TabMain settings={settings} setSettings={vi.fn()} setPresentExt={vi.fn()} />);

    const resetButton = screen.getByRole('button', {name: 'Reset'});
    await waitFor(() => expect(resetButton).toBeEnabled());
    fireEvent.click(resetButton);

    expect(await screen.findByText('Device command sent')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  test('reports a busy device for a power command', async () => {
    mockDevice({'/cmd/~T2': {status: 200, data: 'Busy'}});

    render(<TabMain settings={settings} setSettings={vi.fn()} setPresentExt={vi.fn()} />);

    const powerButton = screen.getByRole('button', {name: 'Power'});
    await waitFor(() => expect(powerButton).toBeEnabled());
    fireEvent.click(powerButton);

    expect(await screen.findByRole('alert')).toHaveTextContent('Device is busy');
  });

  test('resumes heartbeats when the device timer is off', async () => {
    const setSettings = vi.fn();
    mockDevice({'/cmd/~P0': {status: 200, data: 'Invalid'}});

    render(<TabMain settings={{...settings, Pause: true}} setSettings={setSettings} setPresentExt={vi.fn()} />);

    const pauseToggle = screen.getByRole('checkbox', {name: 'Pause'});
    await waitFor(() => expect(pauseToggle).toBeEnabled());
    fireEvent.click(pauseToggle);

    await waitFor(() => expect(setSettings).toHaveBeenCalledWith({Pause: false}));
    expect(screen.getByText('Heartbeat resumed. The device timer is off.')).toBeInTheDocument();
  });

  test('does not pause when the device timer is off', async () => {
    const setSettings = vi.fn();
    mockDevice({'/cmd/~P1': {status: 200, data: 'Invalid'}});

    render(<TabMain settings={settings} setSettings={setSettings} setPresentExt={vi.fn()} />);

    const pauseToggle = screen.getByRole('checkbox', {name: 'Pause'});
    await waitFor(() => expect(pauseToggle).toBeEnabled());
    fireEvent.click(pauseToggle);

    expect(await screen.findByRole('alert')).toHaveTextContent('The device timer is off. There is nothing to pause.');
    expect(setSettings).not.toHaveBeenCalled();
  });

  test('rejects a malformed switch response without changing settings', async () => {
    const setSettings = vi.fn();
    axios.post.mockImplementation((url) => {
      switch (url) {
        case '/cmd/~I': return Promise.resolve({data: '~IWatchDog'});
        case '/cmd/~U': return Promise.resolve({data: '~A'});
        case '/cmd/~G': return Promise.resolve({data: '23.5'});
        case '/cmd/~L0': return Promise.resolve({data: '~L'});
        default: return Promise.reject(new Error(`unexpected URL: ${url}`));
      }
    });

    render(<TabMain settings={settings} setSettings={setSettings} setPresentExt={vi.fn()} />);

    const diodeToggle = screen.getByRole('checkbox', {name: 'Led'});
    await waitFor(() => expect(diodeToggle).toBeEnabled());
    fireEvent.click(diodeToggle);

    expect(await screen.findByRole('alert')).toHaveTextContent('Device command failed');
    expect(setSettings).not.toHaveBeenCalled();
  });
});
