import { config } from 'dotenv';
config();
import fs from 'fs';
import path from 'path';
import axios from 'axios';
import pLimit from 'p-limit';
import { globby } from 'globby';


const currentDir = process.cwd();
console.log('Current Directory:', currentDir);


// Configure options
const options = {
  storageZoneName: "tcno",
  cleanDestination: true,
  accessKey: process.env.FTP_PASSWORD,
  maxConcurrentUploads: 10,
};

// Function to delete a file or directory (assuming deletion is supported for directories)
export const deleteFile = async (targetDirectory, options) => {
  console.log(`DELETE: ${options.storageZoneName}/${targetDirectory}`);
  const url = `https://uk.storage.bunnycdn.com/${options.storageZoneName}/${targetDirectory}`;
  await axios.delete(url, {
    headers: {
      'AccessKey': options.accessKey,
    },
  });
};

// Function to upload a file
export const uploadFile = async (sourcePath, targetPath, options) => {
  const url = `https://uk.storage.bunnycdn.com/${options.storageZoneName}/${targetPath}`;
  console.log(`UPLOAD: /${options.storageZoneName}/${targetPath}`);
  const fileContent = fs.createReadStream(sourcePath);
  await axios.put(url, fileContent, {
    headers: {
      'AccessKey': options.accessKey,
      'Content-Type': 'application/octet-stream',
    },
  });
};

// Function to upload a directory recursively
export const uploadDirectory = async (sourceDirectory, targetDirectory, options = {}) => {
  console.log(`UPLOAD DIR: sourceDirectory [${sourceDirectory}] to targetDirectory [${targetDirectory}]`);
  options = {
    maxConcurrentUploads: 10,
    ...options,
  };

  if (options.cleanDestination) {
    console.log(`Cleaning dest`);
    await deleteFile(targetDirectory, options).catch(() => {});
  }

  console.log(`Starting uploads`);
  options.limit = options.limit || pLimit(options.maxConcurrentUploads);

  const absoluteSourceDirectory = `${path.resolve(sourceDirectory).replace(/\\/g, '/')}`;
  console.log(`Absolute Source Directory: ${absoluteSourceDirectory}`);
  const filePaths = await globby(`${absoluteSourceDirectory}/**/*`, { onlyFiles: true, absolute: true });
  console.log(`Glob Pattern: ${absoluteSourceDirectory}/**/*`);
  console.log(`Files found: ${filePaths.length}`);
  
  if (filePaths.length === 0) {
    console.log('No files found to upload.');
    return;
  }

  await Promise.all(
    filePaths.map(async (sourcePath) => {
      const targetPath = path.join(targetDirectory, path.relative(sourceDirectory, sourcePath));
      return options.limit(() => uploadFile(sourcePath, targetPath, options));
    }),
  );
};

async function uploadFiles() {
  try {
    // detect branch / tag and choose target
    const branchName = (process.env.APPVEYOR_REPO_BRANCH || process.env.BRANCH_NAME || process.env.GIT_BRANCH || '').toLowerCase();
    // APPVEYOR_REPO_TAG can be the string "false" for non-tag builds - check explicitly for "true"
    const isTag = (String(process.env.APPVEYOR_REPO_TAG || '').toLowerCase() === 'true') || !!(process.env.APPVEYOR_REPO_TAG_NAME && process.env.APPVEYOR_REPO_TAG_NAME.trim());
    const isBeta = branchName === 'beta' || branchName.includes('beta');

    // Helpful debug output for CI logs
    console.log(`Env: APPVEYOR_REPO_TAG=${process.env.APPVEYOR_REPO_TAG}, APPVEYOR_REPO_TAG_NAME=${process.env.APPVEYOR_REPO_TAG_NAME}, APPVEYOR_REPO_BRANCH=${process.env.APPVEYOR_REPO_BRANCH}`);

    let latestDir;
    if (isTag) {
      latestDir = 'Projects/AccSwitcher/latest';
    } else if (isBeta) {
      latestDir = 'Projects/AccSwitcher/latest_beta';
    } else {
      console.log(`Skipping upload: branch="${branchName}" and not a tag.`);
      return;
    }

    const dateVersion = process.env.DATEVERSION || process.env.APPVEYOR_BUILD_VERSION || new Date().toISOString().replace(/[:.]/g, '-');
    console.log(`Branch: ${branchName || 'unknown'}, isTag: ${isTag}, uploading to ${latestDir}, dateVersion: ${dateVersion}`);

    await uploadDirectory(
      "TcNo-Acc-Switcher-Client\\bin\\x64\\Release\\TcNo-Acc-Switcher",
      latestDir,
      options
    );
    console.log("All files and folders uploaded successfully.");

    // Upload Hashes
    await uploadFile(
      'TcNo-Acc-Switcher-Client\\bin\\x64\\Release\\UpdateOutput\\hashes.json',
      `${latestDir}/hashes.json`,
      options
    );
    console.log("hashes.json uploaded.");

    await uploadFile(
      'TcNo-Acc-Switcher-Client\\bin\\x64\\Release\\UpdateDiff.7z',
      `Projects/AccSwitcher/updates/${dateVersion}.7z`,
      options
    );

    // If this is a beta build, also upload the built artifact found under the upload folder
    if (isBeta) {
      try {
        const artifactPattern = `${path.resolve('TcNo-Acc-Switcher-Client\\bin\\x64\\Release\\upload\\*.7z').replace(/\\/g, '/')}`;
        console.log(`Looking for artifacts with pattern: ${artifactPattern}`);
        const artifacts = await globby(artifactPattern, { onlyFiles: true, absolute: true });
        console.log(`Artifacts found: ${artifacts.length}`);
        if (artifacts.length > 0) {
          const artifactPath = artifacts[0];
          console.log(`Uploading beta artifact ${artifactPath} as TcNo-Acc-Switcher_Beta.7z`);
          await uploadFile(artifactPath, `Projects/AccSwitcher/updates/TcNo-Acc-Switcher_Beta.7z`, options);
          console.log('Beta artifact uploaded.');
        } else {
          console.log('No beta artifact found to upload.');
        }
      } catch (err) {
        console.error('Error while uploading beta artifact:', err);
      }
    }
  } catch (error) {
    console.error('Error during upload:', error);
    process.exit(1);
  }
}

uploadFiles();
