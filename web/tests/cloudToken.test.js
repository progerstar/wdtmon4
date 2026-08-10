import { isCloudDeviceId, isCloudWriteToken, safeDashboardUrl } from '../src/cloudToken';

test('accepts only a complete Cloud Lite write token', () => {
  const validToken = `utx1_${'a'.repeat(86)}`;

  expect(isCloudWriteToken(validToken)).toBe(true);
  expect(isCloudWriteToken('utx1_token')).toBe(false);
  expect(isCloudWriteToken(`${validToken}!`)).toBe(false);
});

test('accepts only valid cloud device IDs', () => {
  expect(isCloudDeviceId('device-1_A')).toBe(true);
  expect(isCloudDeviceId('')).toBe(false);
  expect(isCloudDeviceId('device name')).toBe(false);
  expect(isCloudDeviceId('a'.repeat(65))).toBe(false);
});

test('accepts only absolute HTTP dashboard URLs without credentials', () => {
  expect(safeDashboardUrl('https://cloud.unitx.pro/dashboard#token')).toBe(
    'https://cloud.unitx.pro/dashboard#token',
  );
  expect(safeDashboardUrl('http://127.0.0.1:8080/dashboard')).toBe(
    'http://127.0.0.1:8080/dashboard',
  );
  expect(safeDashboardUrl('/dashboard')).toBe('');
  expect(safeDashboardUrl('javascript:alert(1)')).toBe('');
  expect(safeDashboardUrl('https://user:password@cloud.unitx.pro/')).toBe('');
});
