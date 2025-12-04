// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2025 TroubleChute (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

fn main() {
    slint_build::compile("ui/main.slint").unwrap();
    
    #[cfg(windows)]
    {
        // Embed the manifest to enable ComCtl32 v6 (required for GetWindowSubclass)
        let manifest = include_str!("updater.manifest");
        let mut res = winres::WindowsResource::new();
        res.set_manifest(manifest);
        res.compile().unwrap();
    }
}

