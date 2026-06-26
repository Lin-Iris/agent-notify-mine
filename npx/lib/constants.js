const path = require('node:path');
const os = require('node:os');

const REPO_OWNER = 'Lin-Iris';
const REPO_NAME = 'agent-notify-mine';
const INSTALL_DIR = path.join(os.homedir(), '.agent-notify');
const TMP_DIR = path.join(INSTALL_DIR, 'tmp');

function getInstalledBinaryPath(target) {
  return path.join(INSTALL_DIR, `agent-notify${target.ext}`);
}

module.exports = {
  REPO_OWNER,
  REPO_NAME,
  INSTALL_DIR,
  TMP_DIR,
  getInstalledBinaryPath,
};
