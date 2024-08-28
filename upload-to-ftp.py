import os
import ftplib
import argparse

def upload_to_ftp(ftp_server, ftp_username, ftp_password, local_folder_path, ftp_folder_path):
    try:
        # Connect to the FTP server
        ftp = ftplib.FTP(ftp_server)
        ftp.login(ftp_username, ftp_password)

        # Function to upload files and folders recursively
        def upload_directory(local_path, ftp_path):
            # Change to the target directory on the FTP server
            try:
                ftp.cwd(ftp_path)
            except ftplib.error_perm:
                # If the directory doesn't exist, create it
                ftp.mkd(ftp_path)
                ftp.cwd(ftp_path)

            # Upload each file in the local directory
            for item in os.listdir(local_path):
                local_item_path = os.path.join(local_path, item)
                ftp_item_path = os.path.join(ftp_path, item)

                if os.path.isdir(local_item_path):
                    # Recursively upload directories
                    upload_directory(local_item_path, ftp_item_path)
                else:
                    # Upload files
                    with open(local_item_path, 'rb') as file:
                        ftp.storbinary(f'STOR {ftp_item_path}', file)
                        print(f'Uploaded {local_item_path} to {ftp_item_path}')

        # Start uploading from the base directory
        upload_directory(local_folder_path, ftp_folder_path)

        # Close the FTP connection
        ftp.quit()
        print("All files and folders uploaded successfully.")

    except ftplib.all_errors as e:
        print(f'FTP error: {e}')

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Upload files and folders to FTP server recursively.')
    parser.add_argument('-ftpServer', required=True, help='FTP server address')
    parser.add_argument('-ftpUsername', required=True, help='FTP username')
    parser.add_argument('-ftpPassword', required=True, help='FTP password')
    parser.add_argument('-localFolderPath', required=True, help='Local folder path containing files and folders to upload')
    parser.add_argument('-ftpFolderPath', required=True, help='FTP folder path where files and folders should be uploaded')

    args = parser.parse_args()

    upload_to_ftp(args.ftpServer, args.ftpUsername, args.ftpPassword, args.localFolderPath, args.ftpFolderPath)
