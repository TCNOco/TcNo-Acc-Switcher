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

slint::include_modules!();

use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;
use std::env;
use crate::updater::Updater;
use crate::logger::Logger;

pub struct UpdaterApp {
    ui: MainWindow,
    status: Arc<Mutex<String>>,
    log_lines: Arc<Mutex<Vec<String>>>,
    progress: Arc<Mutex<f32>>,
    button_text: Arc<Mutex<String>>,
    button_enabled: Arc<Mutex<bool>>,

    verify_mode: Arc<Mutex<bool>>,
    current_version: Arc<Mutex<String>>,
    #[cfg(windows)]
    drag_start: Arc<Mutex<Option<(i32, i32)>>>, // (offset_x, offset_y) - mouse position relative to window
    #[cfg(windows)]
    window_start: Arc<Mutex<Option<(i32, i32)>>>, // (window_x, window_y) when drag started
    #[cfg(windows)]
    drag_initialized: Arc<Mutex<bool>>, // Track if drag has been initialized (to prevent jumping on first move)
}

impl UpdaterApp {
    pub fn new() -> Result<Self, Box<dyn std::error::Error>> {
        // Force software rendering to avoid OpenGL requirement
        std::env::set_var("SLINT_BACKEND", "software");
        
        let ui = MainWindow::new().map_err(|e| format!("Failed to create MainWindow: {}", e))?;
        
        // Shared state
        let status = Arc::new(Mutex::new("Initializing...".to_string()));
        let log_lines = Arc::new(Mutex::new(Vec::new()));
        let progress = Arc::new(Mutex::new(0.0));
        let button_text = Arc::new(Mutex::new("...".to_string()));
        let button_enabled = Arc::new(Mutex::new(false));
        let verify_mode = Arc::new(Mutex::new(false));
        let current_version = Arc::new(Mutex::new(String::new()));
        #[cfg(windows)]
        let drag_start = Arc::new(Mutex::new(None));
        #[cfg(windows)]
        let window_start = Arc::new(Mutex::new(None));
        #[cfg(windows)]
        let drag_initialized = Arc::new(Mutex::new(false));
        
        // Clone for thread
        let status_clone = status.clone();
        let button_text_clone = button_text.clone();
        let button_enabled_clone = button_enabled.clone();
        
        // Setup callbacks
        ui.on_start_button_clicked({
            let button_text_btn = button_text.clone();
            let verify_mode_btn = verify_mode.clone();
            move || {
                let btn_text = button_text_btn.lock().unwrap().clone();
                if btn_text == "Start update" {
                    if let Ok(mut vm) = verify_mode_btn.lock() {
                        *vm = false;
                    }
                    // TODO: Implement update logic
                } else if btn_text == "Verify files" {
                    if let Ok(mut vm) = verify_mode_btn.lock() {
                        *vm = true;
                    }
                    // TODO: Implement verify logic
                } else if btn_text == "Exit" {
                    std::process::exit(0);
                }
            }
        });
        
        ui.on_minimize_window({
            let ui_weak = ui.as_weak();
            move || {
                if let Some(ui) = ui_weak.upgrade() {
                    eprintln!("Minimize button clicked");
                    ui.window().set_minimized(true);
                }
            }
        });
        
        ui.on_close_window({
            move || {
                eprintln!("Close button clicked - exiting");
                std::process::exit(0);
            }
        });
        
        #[cfg(windows)]
        let drag_start_clone = drag_start.clone();
        #[cfg(windows)]
        let window_start_clone = window_start.clone();
        #[cfg(windows)]
        let drag_initialized_clone = drag_initialized.clone();
        ui.on_drag_window({
            #[cfg(windows)]
            {
                use windows::{
                    Win32::Foundation::{HWND, POINT, RECT},
                    Win32::UI::WindowsAndMessaging::{
                        GetCursorPos, GetForegroundWindow, GetWindowRect, SetWindowPos, HWND_TOP,
                        SWP_NOSIZE, SWP_NOZORDER,
                    },
                };
                let drag_start = drag_start_clone.clone();
                let window_start = window_start_clone.clone();
                let drag_initialized = drag_initialized_clone.clone();
                move || {
                    unsafe {
                        let hwnd = GetForegroundWindow();
                        if hwnd.is_invalid() {
                            return;
                        }
                        
                        let mut cursor_pos = POINT::default();
                        if GetCursorPos(&mut cursor_pos).is_err() {
                            return;
                        }
                        
                        let mut window_rect = RECT::default();
                        if GetWindowRect(hwnd, &mut window_rect).is_err() {
                            return;
                        }
                        
                        let mut drag_start_guard = drag_start.lock().unwrap();
                        let mut window_start_guard = window_start.lock().unwrap();
                        let mut drag_initialized_guard = drag_initialized.lock().unwrap();
                        
                        // Check if this is a mouse up event that should reset the drag
                        // We detect this by checking if we have an active drag but the window hasn't moved
                        // This is a heuristic - if offset matches and window position matches, likely mouse up
                        match *drag_start_guard {
                            Some((offset_x, offset_y)) => {
                                let offset_x: i32 = offset_x;
                                let offset_y: i32 = offset_y;
                                let current_offset_x: i32 = cursor_pos.x - window_rect.left;
                                let current_offset_y: i32 = cursor_pos.y - window_rect.top;
                                let diff_x: i32 = current_offset_x - offset_x;
                                let diff_y: i32 = current_offset_y - offset_y;
                                let offset_diff_x: i32 = diff_x.abs();
                                let offset_diff_y: i32 = diff_y.abs();
                                
                                match *window_start_guard {
                                    Some((start_x, start_y)) => {
                                        let start_x: i32 = start_x;
                                        let start_y: i32 = start_y;
                                        let pos_diff_x: i32 = (window_rect.left - start_x).abs();
                                        let pos_diff_y: i32 = (window_rect.top - start_y).abs();
                                        
                                        // If offset is very close to stored AND window hasn't moved, likely mouse up
                                        if offset_diff_x <= 3 && offset_diff_y <= 3 && pos_diff_x <= 2 && pos_diff_y <= 2 {
                                            // Reset drag state (mouse up detected)
                                            *drag_start_guard = None;
                                            *window_start_guard = None;
                                            *drag_initialized_guard = false;
                                            return;
                                        }
                                    }
                                    None => {}
                                }
                            }
                            None => {}
                        }
                        
                        // Now handle the actual drag logic
                        match *drag_start_guard {
                            Some((offset_x, offset_y)) => {
                                // Explicitly type the offset values
                                let offset_x: i32 = offset_x;
                                let offset_y: i32 = offset_y;
                                
                                // Check if this is the first move after click
                                if !*drag_initialized_guard {
                                    // First move event - just mark as initialized, don't move window
                                    *drag_initialized_guard = true;
                                    return;
                                }
                                
                                // Mouse has actually moved - update window position
                                // new_x = current_mouse_x - offset_from_click_point
                                // This keeps the window "stuck" to the mouse at the click point
                                let new_x: i32 = cursor_pos.x - offset_x;
                                let new_y: i32 = cursor_pos.y - offset_y;
                                
                                // Always update window position during drag (Windows handles redundant calls efficiently)
                                // This ensures smooth tracking even when mouse moves quickly
                                let _ = SetWindowPos(
                                    hwnd,
                                    HWND_TOP,
                                    new_x,
                                    new_y,
                                    0,
                                    0,
                                    SWP_NOSIZE | SWP_NOZORDER,
                                );
                            }
                            None => {
                                // No active drag - this is the initial click
                                // Record the offset but DO NOT move the window
                                let offset_x: i32 = cursor_pos.x - window_rect.left;
                                let offset_y: i32 = cursor_pos.y - window_rect.top;
                                
                                // Store offset and initial window position
                                *drag_start_guard = Some((offset_x, offset_y));
                                *window_start_guard = Some((window_rect.left, window_rect.top));
                                *drag_initialized_guard = false;
                                
                                // CRITICAL: Do NOT move the window here
                                // Just record the offset and return
                                return;
                            }
                        }
                    }
                }
            }
            #[cfg(not(windows))]
            {
                move || {
                    // Non-Windows platforms - Slint may handle this automatically
                }
            }
        });
        
        // Add explicit reset callback for mouse up
        #[cfg(windows)]
        let drag_start_reset = drag_start.clone();
        #[cfg(windows)]
        let window_start_reset = window_start.clone();
        // We'll handle reset in the Slint mouse up event by calling drag-window
        // and checking if we should reset
        
        // Initialize updater in background thread
        let log_lines_clone = log_lines.clone();
        thread::spawn(move || {
            if let Err(e) = std::panic::catch_unwind(|| {
                let line = "Background thread: Starting updater initialization...";
                Logger::write_line(line);
                if let Ok(mut log) = log_lines_clone.lock() {
                    log.push(line.to_string());
                }
                
                match Updater::new() {
                    Ok(mut updater) => {
                        let line = "Updater initialized successfully";
                        Logger::write_line(line);
                        if let Ok(mut log) = log_lines_clone.lock() {
                            log.push(line.to_string());
                        }
                        
                        // Check for updates
                        let line = "Checking for updates";
                        Logger::write_line(line);
                        if let Ok(mut log) = log_lines_clone.lock() {
                            log.push(line.to_string());
                        }
                        if let Ok(mut status) = status_clone.lock() {
                            *status = "Checking for updates...".to_string();
                        }
                        
                        match updater.get_updates_list() {
                            Ok(updates) => {
                                if updates.is_empty() {
                                    let line = "No updates available";
                                    Logger::write_line(line);
                                    if let Ok(mut log) = log_lines_clone.lock() {
                                        log.push(line.to_string());
                                    }
                                    if let Ok(mut status) = status_clone.lock() {
                                        *status = line.to_string();
                                    }
                                    if let Ok(mut btn_text) = button_text_clone.lock() {
                                        *btn_text = "Exit".to_string();
                                    }
                                    if let Ok(mut btn_enabled) = button_enabled_clone.lock() {
                                        *btn_enabled = true;
                                    }
                                } else {
                                    let line = format!("{} update(s) available", updates.len());
                                    Logger::write_line(&line);
                                    if let Ok(mut log) = log_lines_clone.lock() {
                                        log.push(line.clone());
                                    }
                                    if let Ok(mut status) = status_clone.lock() {
                                        *status = line;
                                    }
                                    if let Ok(mut btn_text) = button_text_clone.lock() {
                                        *btn_text = "Start update".to_string();
                                    }
                                    if let Ok(mut btn_enabled) = button_enabled_clone.lock() {
                                        *btn_enabled = true;
                                    }
                                }
                            }
                            Err(e) => {
                                let line = format!("Error checking for updates: {}", e);
                                Logger::write_line(&line);
                                if let Ok(mut log) = log_lines_clone.lock() {
                                    log.push(line.clone());
                                }
                                if let Ok(mut status) = status_clone.lock() {
                                    *status = format!("Error: {}", e);
                                }
                            }
                        }
                    }
                    Err(e) => {
                        Logger::write_line(&format!("Failed to initialize updater: {}", e));
                        if let Ok(mut status) = status_clone.lock() {
                            *status = format!("Error: {}", e);
                        }
                    }
                }
            }) {
                eprintln!("Background thread panicked: {:?}", e);
                Logger::write_line("Background thread panicked");
            }
        });
        
        // Update UI from state
        let ui_weak = ui.as_weak();
        let status_ui = status.clone();
        let log_lines_ui = log_lines.clone();
        let progress_ui = progress.clone();
        let button_text_ui = button_text.clone();
        let button_enabled_ui = button_enabled.clone();
        thread::spawn(move || {
            loop {
                thread::sleep(Duration::from_millis(100));
                
                if let Some(ui) = ui_weak.upgrade() {
                    let status_val = status_ui.lock().unwrap().clone();
                    let log_lines_val = log_lines_ui.lock().unwrap().clone();
                    let progress_val = *progress_ui.lock().unwrap();
                    let button_text_val = button_text_ui.lock().unwrap().clone();
                    let button_enabled_val = *button_enabled_ui.lock().unwrap();
                    
                    let log_text = log_lines_val.join("\n");
                    
                    ui.set_app_state(AppState {
                        status_text: status_val.into(),
                        log_text: log_text.into(),
                        progress: progress_val,
                        button_text: button_text_val.into(),
                        button_enabled: button_enabled_val,
                    });
                } else {
                    break;
                }
            }
        });
        
        Ok(Self {
            ui,
            status,
            log_lines,
            progress,
            button_text,
            button_enabled,
            verify_mode,
            current_version,
            #[cfg(windows)]
            drag_start,
            #[cfg(windows)]
            window_start,
            #[cfg(windows)]
            drag_initialized,
        })
    }
    
    pub fn show(&self) {
        if let Err(e) = self.ui.window().show() {
            eprintln!("Failed to show window: {}", e);
            std::process::exit(1);
        }
    }
    
    pub fn run(&self) {
        eprintln!("Starting Slint event loop...");
        match self.ui.run() {
            Ok(_) => {
                eprintln!("Slint event loop exited normally");
            }
            Err(e) => {
                eprintln!("Failed to run UI event loop: {}", e);
                eprintln!("Error details: {:?}", e);
                std::process::exit(1);
            }
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn set_status(&self, status: &str) {
        if let Ok(mut s) = self.status.lock() {
            *s = status.to_string();
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn write_line(&self, line: &str) {
        if let Ok(mut lines) = self.log_lines.lock() {
            lines.push(line.to_string());
            // Keep only last 100 lines
            if lines.len() > 100 {
                lines.remove(0);
            }
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn set_progress(&self, progress: f32) {
        if let Ok(mut p) = self.progress.lock() {
            *p = progress;
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn set_button_text(&self, text: &str) {
        if let Ok(mut bt) = self.button_text.lock() {
            *bt = text.to_string();
        }
    }
    
    #[allow(dead_code)] // Public API for future use
    pub fn set_button_enabled(&self, enabled: bool) {
        if let Ok(mut be) = self.button_enabled.lock() {
            *be = enabled;
        }
    }
}
