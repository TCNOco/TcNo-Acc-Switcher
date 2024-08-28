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
  options = {
    maxConcurrentUploads: 10,
    ...options,
  };

  if (options.cleanDestination) {
    await deleteFile(targetDirectory, options).catch(() => {});
  }

  options.limit = options.limit || pLimit(options.maxConcurrentUploads);

  const filePaths = await globby(`${sourceDirectory}/**/*`, { onlyFiles: true, absolute: true });
  await Promise.all(
    filePaths.map(async (sourcePath) => {
      const targetPath = path.join(targetDirectory, path.relative(sourceDirectory, sourcePath));
      return options.limit(() => uploadFile(sourcePath, targetPath, options));
    }),
  );
};

// Start the upload process
uploadDirectory("TcNo-Acc-Switcher-Client\\bin\\x64\\Release\\TcNo-Acc-Switcher",
                "/Projects/AccSwitcher/latest-test",
                options)
  .then(() => console.log("All files and folders uploaded successfully."))
  .catch(error => {
    console.error('Error during upload:', error);
    process.exit(1);
  });

// Upload Hashes
uploadFile('TcNo-Acc-Switcher-Client\\bin\\x64\\Release\\UpdateOutput\\hashes.json',
            '/Projects/AccSwitcher/latest-test/hashes.json', options)
  .then(() => console.log("hashes.json uploaded."))
  .catch(error => {
    console.error('Error during upload:', error);
    process.exit(1);
  });

fs.copyFileSync('TcNo-Acc-Switcher-Client\\bin\\x64\\Release\\UpdateDiff.7z',
                'UpdateDiff.7z');