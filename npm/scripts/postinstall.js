#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');
const http = require('http');

if (process.env.WIRO_CLI_SKIP_DOWNLOAD === '1') {
  process.exit(0);
}

function getPackageVersion() {
  const pkgPath = path.join(__dirname, '..', 'package.json');
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
  return pkg.version;
}

function getAssetName() {
  const platform = process.platform;
  const arch = process.arch;
  const ext = platform === 'win32' ? '.exe' : '';
  return `wiro-${platform}-${arch}${ext}`;
}

function download(url, destPath, redirectDepth = 0) {
  return new Promise((resolve, reject) => {
    if (redirectDepth > 5) {
      reject(new Error('Too many redirects while downloading binary'));
      return;
    }

    const client = url.startsWith('https:') ? https : http;
    const req = client.get(url, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        const redirected = new URL(res.headers.location, url).toString();
        res.resume();
        resolve(download(redirected, destPath, redirectDepth + 1));
        return;
      }

      if (res.statusCode !== 200) {
        res.resume();
        reject(new Error(`Download failed with status ${res.statusCode} (${url})`));
        return;
      }

      const tmpPath = `${destPath}.tmp`;
      const out = fs.createWriteStream(tmpPath, { mode: 0o755 });
      res.pipe(out);
      out.on('finish', () => {
        out.close((err) => {
          if (err) {
            reject(err);
            return;
          }
          fs.renameSync(tmpPath, destPath);
          if (process.platform !== 'win32') {
            fs.chmodSync(destPath, 0o755);
          }
          resolve();
        });
      });
      out.on('error', (err) => {
        res.destroy();
        reject(err);
      });
    });

    req.on('error', reject);
  });
}

async function main() {
  const version = getPackageVersion();
  const asset = getAssetName();

  const defaultBase = `https://github.com/wiro-ai/wiro-cli/releases/download/v${version}`;
  const base = process.env.WIRO_CLI_DOWNLOAD_BASE || defaultBase;
  const url = `${base}/${asset}`;

  const distDir = path.join(__dirname, '..', 'dist');
  if (!fs.existsSync(distDir)) {
    fs.mkdirSync(distDir, { recursive: true });
  }

  const dest = path.join(distDir, asset);
  try {
    await download(url, dest);
    console.log(`Installed Wiro CLI binary: ${asset}`);
  } catch (err) {
    console.error(`Failed to download Wiro CLI binary from ${url}`);
    console.error(err.message);
    process.exit(1);
  }
}

main();
