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
use crate::utils;
use serde_json::Value;

pub struct UpdaterApp {
    ui: MainWindow,
    _timer: slint::Timer, // Keep timer alive to ensure UI updates continue
    status: Arc<Mutex<String>>,
    log_lines: Arc<Mutex<Vec<String>>>,
    progress: Arc<Mutex<f32>>,
    button_text: Arc<Mutex<String>>,
    button_enabled: Arc<Mutex<bool>>,

    #[allow(dead_code)] // Used via clones in callbacks
    verify_mode: Arc<Mutex<bool>>,
    #[allow(dead_code)] // Used via clones in callbacks
    current_version: Arc<Mutex<String>>,
    #[allow(dead_code)] // Used via clones in update button handler
    latest_version: Arc<Mutex<String>>, // Store latest version for update button
    #[cfg(windows)]
    #[allow(dead_code)] // Used via clones in callbacks
    drag_start: Arc<Mutex<Option<(i32, i32)>>>, // (offset_x, offset_y) - mouse position relative to window
    #[cfg(windows)]
    #[allow(dead_code)] // Used via clones in callbacks
    window_start: Arc<Mutex<Option<(i32, i32)>>>, // (window_x, window_y) when drag started
    #[cfg(windows)]
    #[allow(dead_code)] // Used via clones in callbacks
    drag_initialized: Arc<Mutex<bool>>, // Track if drag has been initialized (to prevent jumping on first move)
}

impl UpdaterApp {
    pub fn new() -> Result<Self, Box<dyn std::error::Error>> {
        // Force software rendering to avoid OpenGL requirement
        unsafe {
            std::env::set_var("SLINT_BACKEND", "software");
        }
        
        let ui = MainWindow::new().map_err(|e| format!("Failed to create MainWindow: {}", e))?;
        
        // Shared state
        let status = Arc::new(Mutex::new("Initializing...".to_string()));
        let log_lines = Arc::new(Mutex::new(Vec::new()));
        let progress = Arc::new(Mutex::new(0.0));
        let button_text = Arc::new(Mutex::new("...".to_string()));
        let button_enabled = Arc::new(Mutex::new(false));
        let verify_mode = Arc::new(Mutex::new(false));
        let current_version = Arc::new(Mutex::new(String::new()));
        let latest_version = Arc::new(Mutex::new(String::new()));
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
        let latest_version_clone = latest_version.clone();
        
        // Setup callbacks
        ui.on_start_button_clicked({
            let button_text_btn = button_text.clone();
            let verify_mode_btn = verify_mode.clone();
            let status_btn = status.clone();
            let log_lines_btn = log_lines.clone();
            let progress_btn = progress.clone();
            let button_enabled_btn = button_enabled.clone();
            let latest_version_btn = latest_version.clone();
            move || {
                let btn_text = button_text_btn.lock().unwrap().clone();
                if btn_text == "Start update" {
                    if let Ok(mut vm) = verify_mode_btn.lock() {
                        *vm = false;
                    }
                    // Disable button during update
                    if let Ok(mut btn_enabled) = button_enabled_btn.lock() {
                        *btn_enabled = false;
                    }
                    
                    // Run update in background thread
                    let status_update = status_btn.clone();
                    let log_lines_update = log_lines_btn.clone();
                    let progress_update = progress_btn.clone();
                    let button_text_update = button_text_btn.clone();
                    let button_enabled_update = button_enabled_btn.clone();
                    let latest_version_update = latest_version_btn.clone();
                    
                    std::thread::spawn(move || {
                        match Updater::new() {
                            Ok(mut updater) => {
                                // Set latest version from stored value
                                if let Ok(latest_ver) = latest_version_update.lock() {
                                    if !latest_ver.is_empty() {
                                        updater.set_latest_version(&latest_ver);
                                    }
                                }
                                
                                let status_cb = |s: &str| {
                                    if let Ok(mut status) = status_update.lock() {
                                        *status = s.to_string();
                                    }
                                    Logger::write_line(s);
                                    if let Ok(mut log) = log_lines_update.lock() {
                                        log.push(s.to_string());
                                    }
                                };
                                
                                let log_cb = |s: &str| {
                                    Logger::write_line(s);
                                    if let Ok(mut log) = log_lines_update.lock() {
                                        log.push(s.to_string());
                                    }
                                };
                                
                                let progress_cb = |downloaded: u64, total: u64| {
                                    if total > 0 {
                                        let percent = (downloaded as f64 / total as f64 * 100.0) as f32;
                                        if let Ok(mut progress) = progress_update.lock() {
                                            *progress = percent;
                                        }
                                    }
                                };
                                
                                // Determine if using CEF (check ActiveBrowser in WindowSettings.json)
                                let using_cef = {
                                    let app_data = utils::get_app_data_folder().unwrap_or_default();
                                    let user_data = utils::get_user_data_folder(&app_data).unwrap_or_default();
                                    let window_settings = user_data.join("WindowSettings.json");
                                    
                                    if window_settings.exists() {
                                        if let Ok(content) = utils::read_all_text(&window_settings) {
                                            if let Ok(json) = serde_json::from_str::<Value>(&content) {
                                                if let Some(browser) = json.get("ActiveBrowser").and_then(|v| v.as_str()) {
                                                    browser.to_lowercase() != "webview"
                                                } else {
                                                    true // Default to CEF
                                                }
                                            } else {
                                                true
                                            }
                                        } else {
                                            true
                                        }
                                    } else {
                                        true // Default to CEF
                                    }
                                };
                                
                                match updater.do_update(using_cef, status_cb, log_cb, progress_cb) {
                                    Ok(_) => {
                                        // Update complete - window will close, but set status anyway
                                        if let Ok(mut status) = status_update.lock() {
                                            *status = "Update complete. Window will close...".to_string();
                                        }
                                    }
                                    Err(e) => {
                                        let error_msg = format!("Error during update: {}", e);
                                        Logger::write_line(&error_msg);
                                        if let Ok(mut log) = log_lines_update.lock() {
                                            log.push(error_msg.clone());
                                        }
                                        if let Ok(mut status) = status_update.lock() {
                                            *status = format!("Error: {}", e);
                                        }
                                        if let Ok(mut btn_text) = button_text_update.lock() {
                                            *btn_text = "Start update".to_string();
                                        }
                                        if let Ok(mut btn_enabled) = button_enabled_update.lock() {
                                            *btn_enabled = true;
                                        }
                                    }
                                }
                            }
                            Err(e) => {
                                let error_msg = format!("Failed to initialize updater for update: {}", e);
                                Logger::write_line(&error_msg);
                                if let Ok(mut log) = log_lines_update.lock() {
                                    log.push(error_msg.clone());
                                }
                                if let Ok(mut status) = status_update.lock() {
                                    *status = format!("Error: {}", e);
                                }
                                if let Ok(mut btn_enabled) = button_enabled_update.lock() {
                                    *btn_enabled = true;
                                }
                            }
                        }
                    });
                } else if btn_text == "Verify files" {
                    if let Ok(mut vm) = verify_mode_btn.lock() {
                        *vm = true;
                    }
                    // Disable button during verify
                    if let Ok(mut btn_enabled) = button_enabled_btn.lock() {
                        *btn_enabled = false;
                    }
                    
                    // Run verify in background thread
                    let status_verify = status_btn.clone();
                    let log_lines_verify = log_lines_btn.clone();
                    let progress_verify = progress_btn.clone();
                    let button_text_verify = button_text_btn.clone();
                    let button_enabled_verify = button_enabled_btn.clone();
                    
                    std::thread::spawn(move || {
                        match Updater::new() {
                            Ok(mut updater) => {
                                let status_cb = |s: &str| {
                                    if let Ok(mut status) = status_verify.lock() {
                                        *status = s.to_string();
                                    }
                                    Logger::write_line(s);
                                    if let Ok(mut log) = log_lines_verify.lock() {
                                        log.push(s.to_string());
                                    }
                                };
                                
                                let log_cb = |s: &str| {
                                    Logger::write_line(s);
                                    if let Ok(mut log) = log_lines_verify.lock() {
                                        log.push(s.to_string());
                                    }
                                };
                                
                                let progress_cb = |downloaded: u64, total: u64| {
                                    if total > 0 {
                                        let percent = (downloaded as f64 / total as f64 * 100.0) as f32;
                                        if let Ok(mut progress) = progress_verify.lock() {
                                            *progress = percent;
                                        }
                                    }
                                };
                                
                                match updater.verify_files(status_cb, log_cb, progress_cb) {
                                    Ok(_) => {
                                        // Verify complete - change button to Exit
                                        if let Ok(mut btn_text) = button_text_verify.lock() {
                                            *btn_text = "Exit".to_string();
                                        }
                                        if let Ok(mut btn_enabled) = button_enabled_verify.lock() {
                                            *btn_enabled = true;
                                        }
                                        if let Ok(mut status) = status_verify.lock() {
                                            *status = "Verification complete!".to_string();
                                        }
                                    }
                                    Err(e) => {
                                        let error_msg = format!("Error during verification: {}", e);
                                        Logger::write_line(&error_msg);
                                        if let Ok(mut log) = log_lines_verify.lock() {
                                            log.push(error_msg.clone());
                                        }
                                        if let Ok(mut status) = status_verify.lock() {
                                            *status = format!("Error: {}", e);
                                        }
                                        if let Ok(mut btn_text) = button_text_verify.lock() {
                                            *btn_text = "Verify files".to_string();
                                        }
                                        if let Ok(mut btn_enabled) = button_enabled_verify.lock() {
                                            *btn_enabled = true;
                                        }
                                    }
                                }
                            }
                            Err(e) => {
                                let error_msg = format!("Failed to initialize updater for verify: {}", e);
                                Logger::write_line(&error_msg);
                                if let Ok(mut log) = log_lines_verify.lock() {
                                    log.push(error_msg.clone());
                                }
                                if let Ok(mut status) = status_verify.lock() {
                                    *status = format!("Error: {}", e);
                                }
                                if let Ok(mut btn_enabled) = button_enabled_verify.lock() {
                                    *btn_enabled = true;
                                }
                            }
                        }
                    });
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
                    Win32::Foundation::{POINT, RECT},
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
        
        // Drag reset is handled in the drag-window callback by detecting mouse up events
        
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
                        
                        // Check if version is empty (verify mode)
                        let current_version = updater.current_version();
                        if current_version.is_empty() {
                            let line = "Could not get version. Click \"Verify\" to update & verify files";
                            Logger::write_line(line);
                            if let Ok(mut log) = log_lines_clone.lock() {
                                log.push(line.to_string());
                            }
                            if let Ok(mut status) = status_clone.lock() {
                                *status = "Press button below!".to_string();
                            }
                            if let Ok(mut btn_text) = button_text_clone.lock() {
                                *btn_text = "Verify files".to_string();
                            }
                            if let Ok(mut btn_enabled) = button_enabled_clone.lock() {
                                *btn_enabled = true;
                            }
                            // Store updater for verify mode (we'll need to access it later)
                            // For now, we'll create a new one when verify is clicked
                            return;
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
                                    // Store latest version for update button
                                    if let Ok(mut latest_ver) = latest_version_clone.lock() {
                                        *latest_ver = updater.latest_version().to_string();
                                    }
                                    
                                    // Display individual updates (like C# version)
                                    // Limit to first 20 updates to avoid UI freezing with many updates
                                    let updates_to_show: Vec<_> = updates.iter().take(20).collect();
                                    let total_count = updates.len();
                                    
                                    for (version, changes_json) in updates_to_show {
                                        let changes: Value = serde_json::from_str(changes_json).unwrap_or(Value::Null);
                                        let update_details = if let Some(changes_array) = changes.as_array() {
                                            if let Some(first_change) = changes_array.get(0) {
                                                first_change.as_str().unwrap_or("").to_string()
                                            } else {
                                                "".to_string()
                                            }
                                        } else {
                                            "".to_string()
                                        };
                                        
                                        let line = format!("Update found: {}", version);
                                        Logger::write_line(&line);
                                        if let Ok(mut log) = log_lines_clone.lock() {
                                            log.push(line.clone());
                                        }
                                        
                                        if !update_details.is_empty() {
                                            let detail_line = format!("- {}", update_details);
                                            Logger::write_line(&detail_line);
                                            if let Ok(mut log) = log_lines_clone.lock() {
                                                log.push(detail_line);
                                            }
                                        }
                                        
                                        let empty_line = "";
                                        Logger::write_line(empty_line);
                                        if let Ok(mut log) = log_lines_clone.lock() {
                                            log.push(empty_line.to_string());
                                        }
                                    }
                                    
                                    // If there are more updates, show a message
                                    if total_count > 20 {
                                        let more_line = format!("... and {} more update(s)", total_count - 20);
                                        Logger::write_line(&more_line);
                                        if let Ok(mut log) = log_lines_clone.lock() {
                                            log.push(more_line);
                                        }
                                    }
                                    
                                    // Add separator and summary
                                    let separator = "-------------------------------------------";
                                    Logger::write_line(separator);
                                    if let Ok(mut log) = log_lines_clone.lock() {
                                        log.push(separator.to_string());
                                    }
                                    
                                    let line = format!("Total updates found: {}", total_count);
                                    Logger::write_line(&line);
                                    if let Ok(mut log) = log_lines_clone.lock() {
                                        log.push(line.clone());
                                    }
                                    
                                    let separator2 = "-------------------------------------------";
                                    Logger::write_line(separator2);
                                    if let Ok(mut log) = log_lines_clone.lock() {
                                        log.push(separator2.to_string());
                                    }
                                    
                                    let click_line = "Click the button below to start the update.";
                                    Logger::write_line(click_line);
                                    if let Ok(mut log) = log_lines_clone.lock() {
                                        log.push(click_line.to_string());
                                    }
                                    
                                    // Update UI state immediately
                                    if let Ok(mut status) = status_clone.lock() {
                                        *status = "Waiting for user input...".to_string();
                                    }
                                    if let Ok(mut btn_text) = button_text_clone.lock() {
                                        *btn_text = "Start update".to_string();
                                    }
                                    if let Ok(mut btn_enabled) = button_enabled_clone.lock() {
                                        *btn_enabled = true;
                                    }
                                    
                                    eprintln!("Updates found: {}, button enabled: true", total_count);
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
        
        // Update UI from state using a timer callback
        // We'll set up a callback that updates the UI periodically from the main thread
        let ui_weak_timer = ui.as_weak();
        let status_timer = status.clone();
        let log_lines_timer = log_lines.clone();
        let progress_timer = progress.clone();
        let button_text_timer = button_text.clone();
        let button_enabled_timer = button_enabled.clone();
        
        // Use Slint's Timer to update UI from the main thread (required for thread safety)
        let timer = slint::Timer::default();
        timer.start(slint::TimerMode::Repeated, Duration::from_millis(100), move || {
            if let Some(ui) = ui_weak_timer.upgrade() {
                let status_val = status_timer.lock().unwrap().clone();
                let log_lines_val = log_lines_timer.lock().unwrap().clone();
                let progress_val = *progress_timer.lock().unwrap();
                let button_text_val = button_text_timer.lock().unwrap().clone();
                let button_enabled_val = *button_enabled_timer.lock().unwrap();
                
                let log_text = log_lines_val.join("\n");
                
                // Update the app-state property directly
                ui.set_app_state(AppState {
                    status_text: status_val.into(),
                    log_text: log_text.into(),
                    progress: progress_val,
                    button_text: button_text_val.into(),
                    button_enabled: button_enabled_val,
                });
            }
        });
        
        Ok(Self {
            ui,
            _timer: timer, // Keep timer alive
            status,
            log_lines,
            progress,
            button_text,
            button_enabled,
            verify_mode,
            current_version,
            latest_version,
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
