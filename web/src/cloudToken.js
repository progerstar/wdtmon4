const CLOUD_WRITE_TOKEN = /^utx1_[A-Za-z0-9_-]{86}$/;
const CLOUD_DEVICE_ID = /^[A-Za-z0-9_-]{1,64}$/;

export function isCloudWriteToken(token) {
  return CLOUD_WRITE_TOKEN.test((token || "").trim());
}

export function isCloudDeviceId(deviceId) {
  return CLOUD_DEVICE_ID.test((deviceId || "").trim());
}

export function safeDashboardUrl(value) {
  if (typeof value !== "string" || value.trim() === "") return "";

  try {
    const url = new URL(value.trim());
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) {
      return "";
    }
    return url.href;
  } catch {
    return "";
  }
}
