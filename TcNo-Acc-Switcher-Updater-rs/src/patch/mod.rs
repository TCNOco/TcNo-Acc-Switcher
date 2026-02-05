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
use std::io::{self, Read, Cursor};
use std::path::Path;
use crate::utils;

// VCDiff implementation
// The C# version uses VCDiff NuGet package (version 4.0.1) based on open-vcdiff
// We use vcdiff-decoder (pure Rust) for decoding patches (applying updates)
// For encoding (creating patches), we use a fallback that copies the new file
// Encoding is typically done by the C# version when creating updates, so this is acceptable

pub fn do_encode<P: AsRef<Path>, Q: AsRef<Path>, R: AsRef<Path>>(
    old_file: P,
    new_file: Q,
    patch_file_output: R,
) -> Result<(), io::Error> {
    // Read old file (dictionary)
    let mut dict_data = Vec::new();
    File::open(&old_file)?.read_to_end(&mut dict_data)?;
    
    // Read new file (target)
    let mut target_data = Vec::new();
    File::open(&new_file)?.read_to_end(&mut target_data)?;
    
    // For encoding, we need to create VCDiff patches
    // The C# version uses: VcEncoder coder = new(dict, target, output);
    // with Encode(true, ChecksumFormat.SDCH) - encodes with no checksum and not interleaved
    // 
    // Since there's no pure Rust VCDiff encoder readily available, we'll use a fallback:
    // If files are very similar, try to create a simple diff. Otherwise, copy the new file.
    // In practice, encoding is typically done by the C# version when creating updates.
    // 
    // For now, we'll copy the new file as a fallback. A proper encoder would require:
    // 1. FFI bindings to open-vcdiff C++ library, OR
    // 2. A pure Rust VCDiff encoder implementation
    //
    // This fallback ensures the updater can still function, though patches won't be optimal.
    std::fs::copy(new_file, patch_file_output)?;
    
    Ok(())
}

#[allow(dead_code)] // Used by apply_patches
pub fn do_decode<P: AsRef<Path>, Q: AsRef<Path>, R: AsRef<Path>>(
    old_file: P,
    patch_file: Q,
    output_new_file: R,
) -> Result<(), io::Error> {
    // Read old file (dictionary)
    let mut dict_data = Vec::new();
    File::open(&old_file)?.read_to_end(&mut dict_data)?;
    
    // Read patch file (delta)
    let mut patch_data = Vec::new();
    File::open(&patch_file)?.read_to_end(&mut patch_data)?;
    
    // Check if this is actually a VCDiff patch or a full file copy
    // VCDiff format starts with specific magic bytes (0xd6, 0xc3, 0xc4, 0x00)
    // If patch file is larger than old file * 2, it's likely a full copy
    let patch_meta = std::fs::metadata(&patch_file)?;
    let old_meta = std::fs::metadata(&old_file)?;
    
    if patch_meta.len() > old_meta.len() * 2 {
        // Likely a full file copy, not a patch - just copy it
        std::fs::copy(patch_file, output_new_file)?;
        return Ok(());
    }
    
    // Check for VCDiff magic bytes
    if patch_data.len() >= 4 {
        let magic = &patch_data[0..4];
        // VCDiff format magic: 0xd6, 0xc3, 0xc4, 0x00
        if magic != [0xd6, 0xc3, 0xc4, 0x00] {
            // Not a VCDiff patch, likely a full file copy
            std::fs::copy(patch_file, output_new_file)?;
            return Ok(());
        }
    }
    
    // Create VCDiff decoder using pure Rust implementation
    // The C# version uses: VcDecoder decoder = new(dict, target, output);
    // with Decode(out _) - the header must be available before first call
    // vcdiff-decoder provides apply_patch(patch, src, sink) where:
    // - patch: Read+Seek containing patch data
    // - src: Option<Read+Seek> containing source (dictionary) data
    // - sink: Write that receives the patched data
    let mut patch_cursor = Cursor::new(patch_data);
    let mut source_cursor = Cursor::new(dict_data);
    let mut output_buffer = Vec::new();
    
    vcdiff_decoder::apply_patch(&mut patch_cursor, Some(&mut source_cursor), &mut output_buffer)
        .map_err(|e| io::Error::new(io::ErrorKind::Other, format!("VCDiff decoding failed: {}", e)))?;
    
    // Write decoded file to output
    std::fs::write(output_new_file, output_buffer)?;
    
    Ok(())
}

#[allow(dead_code)] // Used by updater::apply_update
pub fn apply_patches<P: AsRef<Path>, Q: AsRef<Path>>(
    old_folder: P,
    output_folder: Q,
    main_browser: &str,
) -> Result<(), io::Error> {
    let patches_dir = output_folder.as_ref().join("patches");
    if !patches_dir.exists() {
        return Ok(());
    }
    
    let cef_files = vec![
        "libcef.dll",
        "icudtl.dat",
        "resources.pak",
        "libGLESv2.dll",
        "d3dcompiler_47.dll",
        "vk_swiftshader.dll",
        "chrome_elf.dll",
        "CefSharp.BrowserSubprocess.Core.dll",
    ];
    
    let is_cef_file = |file: &Path| -> bool {
        let file_str = file.to_string_lossy().to_lowercase();
        cef_files.iter().any(|&cef| file_str.contains(cef))
    };
    
    for entry in walkdir::WalkDir::new(&patches_dir) {
        let entry = entry?;
        let patch_path = entry.path();
        
        if patch_path.is_dir() {
            continue;
        }
        
        let relative_path = patch_path.strip_prefix(&patches_dir)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidInput, e))?;
        
        let mut relative_str = relative_path.to_string_lossy().to_string();
        
        // Fix for pre 2021-06-28_01 versions
        if relative_str.starts_with("wwwroot") {
            relative_str = relative_str.replace("wwwroot", "originalwwwroot");
        }
        
        // Check if part of CEF, and skip if CEF not selected
        if relative_str.starts_with("runtimes") && is_cef_file(patch_path) {
            if !patch_path.exists() {
                let create_path = output_folder.as_ref().join(&relative_str);
                if let Some(parent) = create_path.parent() {
                    std::fs::create_dir_all(parent)?;
                }
                File::create(&create_path)?;
                continue;
            }
            if main_browser != "CEF" {
                continue;
            }
        }
        
        let old_file_path = old_folder.as_ref().join(&relative_str);
        if !old_file_path.exists() {
            continue;
        }
        
        let patched_file = output_folder.as_ref().join("patched").join(&relative_str);
        if let Some(parent) = patched_file.parent() {
            std::fs::create_dir_all(parent)?;
        }
        
        do_decode(&old_file_path, patch_path, &patched_file)?;
    }
    
    Ok(())
}

pub fn create_folder_patches<P: AsRef<Path>, Q: AsRef<Path>, R: AsRef<Path>>(
    old_folder: P,
    new_folder: Q,
    output_folder: R,
    include_updater: bool,
) -> Result<Vec<String>, io::Error> {
    let output = output_folder.as_ref();
    std::fs::create_dir_all(output)?;
    
    let mut old_dict = std::collections::HashMap::new();
    let mut new_dict = std::collections::HashMap::new();
    let mut all_new_dict = std::collections::HashMap::new();
    
    dir_search_with_hash(&old_folder, &mut old_dict, false)?;
    dir_search_with_hash(&new_folder, &mut new_dict, false)?;
    dir_search_with_hash(&new_folder, &mut all_new_dict, include_updater)?;
    
    // Save dict of new files & hashes
    let hashes_json = serde_json::to_string_pretty(&all_new_dict)?;
    std::fs::write(output.join("hashes.json"), hashes_json)?;
    
    let mut different_files = Vec::new();
    let mut files_to_delete = Vec::new();
    
    // Compare the 2 folders
    let old_keys: Vec<String> = old_dict.keys().cloned().collect();
    for key in old_keys {
        if let Some(new_hash) = new_dict.remove(&key) {
            if let Some(old_hash) = old_dict.remove(&key) {
                if old_hash != new_hash {
                    different_files.push(key.clone());
                }
            }
        } else {
            // New dictionary does NOT have this file
            files_to_delete.push(key.clone());
            old_dict.remove(&key);
        }
    }
    
    // Handle new files
    let output_new_folder = output.join("new");
    std::fs::create_dir_all(&output_new_folder)?;
    
    for (new_file, _) in &new_dict {
        let new_file_input = new_folder.as_ref().join(new_file);
        let new_file_output = output_new_folder.join(new_file);
        
        if let Some(parent) = new_file_output.parent() {
            std::fs::create_dir_all(parent)?;
        }
        std::fs::copy(&new_file_input, &new_file_output)?;
    }
    
    // Handle different files - create patches
    let patches_dir = output.join("patches");
    std::fs::create_dir_all(&patches_dir)?;
    
    for different_file in &different_files {
        let old_file_input = old_folder.as_ref().join(different_file);
        let new_file_input = new_folder.as_ref().join(different_file);
        let patch_file_output = patches_dir.join(different_file);
        
        if let Some(parent) = patch_file_output.parent() {
            std::fs::create_dir_all(parent)?;
        }
        
        do_encode(&old_file_input, &new_file_input, &patch_file_output)?;
    }
    
    Ok(files_to_delete)
}

fn dir_search_with_hash<P: AsRef<Path>>(
    s_dir: P,
    dict: &mut std::collections::HashMap<String, String>,
    include_updater: bool,
) -> Result<(), io::Error> {
    let dir = s_dir.as_ref();
    
    for entry in walkdir::WalkDir::new(dir) {
        let entry = entry?;
        let path = entry.path();
        
        if path.is_file() {
            if !include_updater {
                let path_str = path.to_string_lossy();
                if path_str.contains("updater") && path_str.contains(std::path::MAIN_SEPARATOR) {
                    let parts: Vec<&str> = path_str.split(std::path::MAIN_SEPARATOR).collect();
                    if parts.iter().any(|&p| p == "updater") {
                        continue;
                    }
                }
            }
            
            let relative = path.strip_prefix(dir)
                .map_err(|e| io::Error::new(io::ErrorKind::InvalidInput, e))?;
            let relative_str = relative.to_string_lossy().replace("/", "\\");
            
            let hash = utils::get_file_md5(path)?;
            dict.insert(relative_str, hash);
        }
    }
    
    Ok(())
}

