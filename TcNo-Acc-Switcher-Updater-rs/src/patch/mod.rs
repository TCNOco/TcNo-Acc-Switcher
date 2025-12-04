// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2025 TroubleChute (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

use std::fs::File;
use std::io;
use std::path::Path;
use crate::utils;

// VCDiff implementation
// NOTE: This requires FFI bindings to the VCDiff C++ library or a Rust implementation
// The C# version uses VCDiff NuGet package (version 4.0.1)
// For now, we detect if the patch file is actually a full file copy (when patch == new file size)
// and handle it accordingly. Proper VCDiff implementation would require:
// 1. FFI bindings to open-vcdiff C++ library, OR
// 2. A pure Rust VCDiff implementation

pub fn do_encode<P: AsRef<Path>, Q: AsRef<Path>, R: AsRef<Path>>(
    old_file: P,
    new_file: Q,
    patch_file_output: R,
) -> Result<(), io::Error> {
    // TODO: Implement proper VCDiff encoding using open-vcdiff library via FFI
    // For now, if files are very different in size, copy the new file
    // Otherwise, we'd create a proper VCDiff patch
    
    let old_meta = std::fs::metadata(&old_file)?;
    let new_meta = std::fs::metadata(&new_file)?;
    
    // If files are significantly different, just copy new file
    // This is a fallback until proper VCDiff is implemented
    let size_diff = if old_meta.len() > new_meta.len() {
        old_meta.len() - new_meta.len()
    } else {
        new_meta.len() - old_meta.len()
    };
    
    if size_diff > old_meta.len() / 2 {
        std::fs::copy(new_file, patch_file_output)?;
    } else {
        // For similar-sized files, we'd create a VCDiff patch
        // For now, copy new file as placeholder
        std::fs::copy(new_file, patch_file_output)?;
    }
    
    Ok(())
}

#[allow(dead_code)] // Used by apply_patches
pub fn do_decode<P: AsRef<Path>, Q: AsRef<Path>, R: AsRef<Path>>(
    old_file: P,
    patch_file: Q,
    output_new_file: R,
) -> Result<(), io::Error> {
    // TODO: Implement proper VCDiff decoding using open-vcdiff library via FFI
    // Check if patch file is actually a VCDiff patch or a full file copy
    // VCDiff patches typically start with specific magic bytes
    
    let patch_meta = std::fs::metadata(&patch_file)?;
    let old_meta = std::fs::metadata(&old_file)?;
    
    // Try to detect if this is a VCDiff patch or a full file
    // VCDiff format has specific header structure
    let mut patch_reader = std::fs::File::open(&patch_file)?;
    let mut header = [0u8; 4];
    let _ = std::io::Read::read(&mut patch_reader, &mut header);
    
    // If patch file size is similar to old file, it might be a full copy
    // Otherwise, try to apply as VCDiff (for now, just copy patch as placeholder)
    if patch_meta.len() > old_meta.len() * 2 {
        // Likely a full file copy, not a patch
        std::fs::copy(patch_file, output_new_file)?;
    } else {
        // Would apply VCDiff patch here
        // For now, copy patch file as placeholder
        // Proper implementation would:
        // 1. Read old_file as dictionary
        // 2. Read patch_file as delta
        // 3. Apply delta to dictionary to create output_new_file
        std::fs::copy(patch_file, output_new_file)?;
    }
    
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

