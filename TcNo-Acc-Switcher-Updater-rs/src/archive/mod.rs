// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2025 TroubleChute (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

use std::fs::File;
use std::io::{BufReader, Write};
use std::path::Path;

// Note: sevenz-rust API may differ, using a simplified approach
pub fn extract_archive<P: AsRef<Path>, Q: AsRef<Path>>(
    archive_path: P,
    output_dir: Q,
) -> Result<(), Box<dyn std::error::Error>> {
    let archive = archive_path.as_ref();
    let output = output_dir.as_ref();
    
    std::fs::create_dir_all(output)?;
    
    // Use sevenz-rust to extract
    let file = File::open(archive)?;
    // sevenz-rust 0.1.5 Password type is public but module is private
    // Try using Default::default() to create an empty password
    let password = Default::default();
    let mut archive_reader = sevenz_rust::SevenZReader::new(
        BufReader::new(file),
        0, // max_size
        password,
    )?;
    
    archive_reader.for_each_entries(|entry, reader| -> Result<bool, sevenz_rust::Error> {
        let entry_path = output.join(&entry.name);
        
        if entry.is_directory {
            std::fs::create_dir_all(&entry_path)
                .map_err(|e| sevenz_rust::Error::Io(e))?;
        } else {
            if let Some(parent) = entry_path.parent() {
                std::fs::create_dir_all(parent)
                    .map_err(|e| sevenz_rust::Error::Io(e))?;
            }
            
            let mut out_file = File::create(&entry_path)
                .map_err(|e| sevenz_rust::Error::Io(e))?;
            let mut buffer = vec![0u8; 8192];
            loop {
                let bytes_read = reader.read(&mut buffer)
                    .map_err(|e| sevenz_rust::Error::Io(e))?;
                if bytes_read == 0 {
                    break;
                }
                out_file.write_all(&buffer[..bytes_read])
                    .map_err(|e| sevenz_rust::Error::Io(e))?;
            }
        }
        
        Ok(true)
    })?;
    
    Ok(())
}

