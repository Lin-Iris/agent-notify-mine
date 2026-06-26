const test = require('node:test');
const assert = require('node:assert/strict');
const { getPlatformTarget } = require('../lib/platform');
const { buildAssetName, buildDownloadUrl } = require('../lib/release');

test('maps darwin arm64 to release target', () => {
  assert.deepEqual(getPlatformTarget({ platform: 'darwin', arch: 'arm64' }), {
    goos: 'darwin',
    goarch: 'arm64',
    ext: '',
  });
});

test('maps win32 x64 to release target', () => {
  assert.deepEqual(getPlatformTarget({ platform: 'win32', arch: 'x64' }), {
    goos: 'windows',
    goarch: 'amd64',
    ext: '.exe',
  });
});

test('maps win32 arm64 to release target', () => {
  assert.deepEqual(getPlatformTarget({ platform: 'win32', arch: 'arm64' }), {
    goos: 'windows',
    goarch: 'arm64',
    ext: '.exe',
  });
});

test('throws on unsupported platform', () => {
  assert.throws(() => getPlatformTarget({ platform: 'freebsd', arch: 'x64' }), /unsupported platform/);
});

test('throws on unsupported architecture', () => {
  assert.throws(() => getPlatformTarget({ platform: 'linux', arch: 'ia32' }), /unsupported platform/);
});

test('builds release asset names from version and target', () => {
  const target = { goos: 'linux', goarch: 'arm64', ext: '' };
  assert.equal(buildAssetName('0.2.3', target), 'agent-notify-v0.2.3-linux-arm64.tar.gz');
});

test('builds download URL from version and asset name', () => {
  assert.equal(
    buildDownloadUrl('0.2.3', 'agent-notify-v0.2.3-linux-arm64.tar.gz'),
    'https://github.com/Lin-Iris/agent-notify-mine/releases/download/v0.2.3/agent-notify-v0.2.3-linux-arm64.tar.gz',
  );
});
