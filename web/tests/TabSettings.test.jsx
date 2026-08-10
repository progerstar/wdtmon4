import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import axios from 'axios';
import { beforeEach, describe, expect, test, vi } from 'vitest';
import TabSettings, { buildSettingsCommand, decodeSettingsResponse } from '../src/TabSettings';

vi.mock('axios', () => ({
  default: {
    post: vi.fn(),
  },
}));

const validResponse = '~F1234512A3FF';

describe('WatchDog settings protocol', () => {
  test('encodes only integer values into a fixed-width command', () => {
    expect(buildSettingsCommand({
      t1: 1,
      t2: 2,
      t3: 3,
      t4: 4,
      t5: 5,
      ch1: 1,
      ch2: 2,
      limit: 10,
      inp: 3,
      temp: 255,
    })).toBe('~W1234512A3FF');

    expect(buildSettingsCommand({
      t1: 1,
      t2: 2,
      t3: 3,
      t4: 4,
      t5: 5,
      ch1: 1,
      ch2: 2,
      limit: 10,
      inp: 3,
      temp: 1.5,
    })).toBeNull();
  });

  test('rejects malformed device responses', () => {
    expect(decodeSettingsResponse(validResponse)).toMatchObject({
      t1: 1,
      t2: 2,
      t3: 3,
      t4: 4,
      t5: 5,
      ch1: 1,
      ch2: 2,
      limit: 10,
      inp: 3,
      temp: 255,
    });
    expect(decodeSettingsResponse('OK')).toBeNull();
    expect(decodeSettingsResponse('~F12GGGGGGGGG')).toBeNull();
    expect(decodeSettingsResponse('~F1234512A3FZ')).toBeNull();
  });
});

describe('TabSettings error feedback', () => {
  beforeEach(() => {
    axios.post.mockReset();
  });

  test('shows a visible error when settings cannot be read', async () => {
    axios.post.mockRejectedValueOnce(new Error('serial timeout'));

    render(<TabSettings />);

    expect(await screen.findByRole('alert')).toHaveTextContent('Settings read failed');
  });

  test('does not report success for an unexpected write response', async () => {
    axios.post
      .mockResolvedValueOnce({data: validResponse})
      .mockResolvedValueOnce({data: 'OK'});

    render(<TabSettings />);

    const writeButton = screen.getByRole('button', {name: 'Write'});
    await waitFor(() => expect(writeButton).toBeEnabled());
    fireEvent.click(writeButton);

    expect(await screen.findByRole('alert')).toHaveTextContent('Unexpected device response');
    expect(screen.queryByText('Settings updated')).not.toBeInTheDocument();
    expect(axios.post).toHaveBeenLastCalledWith(
      '/cmd/~W1234512A3FF',
      undefined,
      expect.objectContaining({timeout: 3000, signal: expect.any(AbortSignal)}),
    );
  });
});
