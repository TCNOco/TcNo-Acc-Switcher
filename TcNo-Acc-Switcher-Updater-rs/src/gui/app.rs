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

use eframe::egui;
use std::sync::{Arc, Mutex};
use std::thread;
use crate::updater::Updater;
use crate::logger::Logger;

pub struct UpdaterApp {
    status: Arc<Mutex<String>>,
    log_lines: Arc<Mutex<Vec<String>>>,
    progress: Arc<Mutex<f32>>,
    button_text: Arc<Mutex<String>>,
    button_enabled: Arc<Mutex<bool>>,
    #[allow(dead_code)] // Reserved for future use
    updates_available: usize,
    theme_colors: ThemeColors,
    verify_mode: Arc<Mutex<bool>>,
    current_version: Arc<Mutex<String>>,
}

#[derive(Clone)]
#[allow(dead_code)] // Fields are used in GUI rendering
struct ThemeColors {
    highlight: egui::Color32,
    header: egui::Color32,
    main_bg: egui::Color32,
    list_bg: egui::Color32,
    border: egui::Color32,
    button_bg: egui::Color32,
    button_bg_hover: egui::Color32,
    button_bg_active: egui::Color32,
    button_color: egui::Color32,
    button_border: egui::Color32,
    button_border_hover: egui::Color32,
    button_border_active: egui::Color32,
}

impl Default for ThemeColors {
    fn default() -> Self {
        Self {
            highlight: egui::Color32::from_rgb(139, 233, 253),
            header: egui::Color32::from_rgb(37, 51, 64),
            main_bg: egui::Color32::from_rgb(14, 20, 25),
            list_bg: egui::Color32::from_rgb(7, 10, 13),
            border: egui::Color32::from_rgb(136, 136, 136),
            button_bg: egui::Color32::from_rgb(39, 69, 96),
            button_bg_hover: egui::Color32::from_rgb(40, 55, 78),
            button_bg_active: egui::Color32::from_rgb(40, 55, 78),
            button_color: egui::Color32::from_rgb(255, 255, 255),
            button_border: egui::Color32::from_rgb(39, 69, 96),
            button_border_hover: egui::Color32::from_rgb(39, 69, 96),
            button_border_active: egui::Color32::from_rgb(139, 233, 253),
        }
    }
}

impl UpdaterApp {
    pub fn new() -> Self {
        let mut app = Self {
            status: Arc::new(Mutex::new("Initializing...".to_string())),
            log_lines: Arc::new(Mutex::new(Vec::new())),
            progress: Arc::new(Mutex::new(0.0)),
            button_text: Arc::new(Mutex::new("...".to_string())),
            button_enabled: Arc::new(Mutex::new(false)),
            updates_available: 0,
            theme_colors: ThemeColors::default(),
            verify_mode: Arc::new(Mutex::new(false)),
            current_version: Arc::new(Mutex::new(String::new())),
        };
        
        // Load theme
        app.load_theme();
        
        // Initialize updater
        thread::spawn({
            let status = app.status.clone();
            let log_lines = app.log_lines.clone();
            let button_text = app.button_text.clone();
            let button_enabled = app.button_enabled.clone();
            let current_version = app.current_version.clone();
            
            move || {
                match Updater::new() {
                    Ok(mut updater) => {
                        *current_version.lock().unwrap() = updater.current_version().to_string();
                        
                        // Check if version is empty (needs verification)
                        if updater.current_version().is_empty() {
                            *status.lock().unwrap() = "Press button below!".to_string();
                            *button_text.lock().unwrap() = "Verify files".to_string();
                            *button_enabled.lock().unwrap() = true;
                            log_lines.lock().unwrap().push("Could not get version. Click \"Verify\" to update & verify files".to_string());
                            return;
                        }
                        
                        *status.lock().unwrap() = "Checking for updates".to_string();
                        Logger::write_line("Checking for updates");
                        
                        match updater.get_updates_list() {
                            Ok(updates) => {
                                let count = updates.len();
                                if count > 0 {
                                    *button_text.lock().unwrap() = "Start update".to_string();
                                    *button_enabled.lock().unwrap() = true;
                                    *status.lock().unwrap() = "Waiting for user input...".to_string();
                                    
                                    // Format updates like C# version
                                    for (version, changes_json) in &updates {
                                        log_lines.lock().unwrap().push(format!("Update found: {}", version));
                                        
                                        // Parse changes JSON to get first change detail
                                        if let Ok(changes_value) = serde_json::from_str::<serde_json::Value>(changes_json) {
                                            if let Some(changes_array) = changes_value.as_array() {
                                                if let Some(first_change) = changes_array.first() {
                                                    if let Some(change_str) = first_change.as_str() {
                                                        log_lines.lock().unwrap().push(format!("- {}", change_str));
                                                    } else if let Some(change_obj) = first_change.as_object() {
                                                        if let Some(change_str) = change_obj.values().next().and_then(|v| v.as_str()) {
                                                            log_lines.lock().unwrap().push(format!("- {}", change_str));
                                                        }
                                                    }
                                                }
                                            }
                                        }
                                        
                                        log_lines.lock().unwrap().push(String::new()); // Empty line
                                        Logger::write_line(&format!("Update found: {}", version));
                                    }
                                    
                                    log_lines.lock().unwrap().push("-------------------------------------------".to_string());
                                    log_lines.lock().unwrap().push(format!("Total updates found: {}", count));
                                    log_lines.lock().unwrap().push("-------------------------------------------".to_string());
                                    log_lines.lock().unwrap().push("Click the button below to start the update.".to_string());
                                } else {
                                    *button_text.lock().unwrap() = "Exit".to_string();
                                    *button_enabled.lock().unwrap() = true;
                                    *status.lock().unwrap() = ":)".to_string();
                                    log_lines.lock().unwrap().push("-------------------------------------------".to_string());
                                    log_lines.lock().unwrap().push("You're up to date.".to_string());
                                }
                            }
                            Err(e) => {
                                *status.lock().unwrap() = format!("Error: {}", e);
                                Logger::write_line(&format!("Error getting updates: {}", e));
                            }
                        }
                    }
                    Err(e) => {
                        *status.lock().unwrap() = format!("Error: {}", e);
                        Logger::write_line(&format!("Error initializing updater: {}", e));
                    }
                }
            }
        });
        
        app
    }
    
    fn load_theme(&mut self) {
        // Try to load theme from WindowSettings.json
        if let Ok(app_data) = crate::utils::get_app_data_folder() {
            if let Ok(user_data) = crate::utils::get_user_data_folder(&app_data) {
                let window_settings = user_data.join("WindowSettings.json");
                if window_settings.exists() {
                    if let Ok(content) = crate::utils::read_all_text(&window_settings) {
                        if let Ok(json) = serde_json::from_str::<serde_json::Value>(&content) {
                            if let Some(theme_name) = json.get("ActiveTheme").and_then(|v| v.as_str()) {
                                let theme_file = user_data.join("themes").join(theme_name).join("info.yaml");
                                if theme_file.exists() {
                                    if let Ok(yaml_content) = crate::utils::read_all_text(&theme_file) {
                                        if let Ok(theme_data) = serde_yaml::from_str::<serde_yaml::Value>(&yaml_content) {
                                            // Parse theme colors from YAML
                                            self.theme_colors = self.parse_theme_colors(&theme_data);
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    
    fn parse_theme_colors(&self, theme_data: &serde_yaml::Value) -> ThemeColors {
        let get_color = |key: &str, default: egui::Color32| -> egui::Color32 {
            if let Some(value) = theme_data.get(key) {
                if let Some(color_str) = value.as_str() {
                    // Parse hex color like "#8BE9FD" or "rgb(139, 233, 253)"
                    if color_str.starts_with('#') && color_str.len() == 7 {
                        if let (Ok(r), Ok(g), Ok(b)) = (
                            u8::from_str_radix(&color_str[1..3], 16),
                            u8::from_str_radix(&color_str[3..5], 16),
                            u8::from_str_radix(&color_str[5..7], 16),
                        ) {
                            return egui::Color32::from_rgb(r, g, b);
                        }
                    }
                }
            }
            default
        };
        
        ThemeColors {
            highlight: get_color("linkColor", egui::Color32::from_rgb(139, 233, 253)),
            header: get_color("headerbarBackground", egui::Color32::from_rgb(37, 51, 64)),
            main_bg: get_color("mainBackground", egui::Color32::from_rgb(14, 20, 25)),
            list_bg: get_color("modalInputBackground", egui::Color32::from_rgb(7, 10, 13)),
            border: get_color("borderedItemBorderColor", egui::Color32::from_rgb(136, 136, 136)),
            button_bg: get_color("buttonBackground", egui::Color32::from_rgb(39, 69, 96)),
            button_bg_hover: get_color("buttonBackground-hover", egui::Color32::from_rgb(40, 55, 78)),
            button_bg_active: get_color("buttonBackground-active", egui::Color32::from_rgb(40, 55, 78)),
            button_color: get_color("buttonColor", egui::Color32::from_rgb(255, 255, 255)),
            button_border: get_color("buttonBorder", egui::Color32::from_rgb(39, 69, 96)),
            button_border_hover: get_color("buttonBorder-hover", egui::Color32::from_rgb(39, 69, 96)),
            button_border_active: get_color("buttonBorder-active", egui::Color32::from_rgb(139, 233, 253)),
        }
    }
    
    #[allow(dead_code)] // Helper method, may be used in future
    fn write_line(&self, line: &str) {
        self.log_lines.lock().unwrap().push(line.to_string());
        Logger::write_line(line);
    }
    
    #[allow(dead_code)] // Helper method, may be used in future
    fn set_status(&self, status: &str) {
        *self.status.lock().unwrap() = status.to_string();
        Logger::write_line(&format!("Status: {}", status));
    }
    
    fn start_update(&mut self) {
        let verify_mode = *self.verify_mode.lock().unwrap();
        *self.button_text.lock().unwrap() = "Started".to_string();
        *self.button_enabled.lock().unwrap() = false;
        
        let status = self.status.clone();
        let log_lines = self.log_lines.clone();
        let progress = self.progress.clone();
        let button_text = self.button_text.clone();
        let button_enabled = self.button_enabled.clone();
        
        thread::spawn(move || {
            let mut updater = match Updater::new() {
                Ok(u) => u,
                Err(e) => {
                    *status.lock().unwrap() = format!("Error: {}", e);
                    return;
                }
            };
            
            let status_cb = |s: &str| {
                *status.lock().unwrap() = s.to_string();
            };
            let log_cb = |s: &str| {
                log_lines.lock().unwrap().push(s.to_string());
                Logger::write_line(s);
            };
            let progress_cb = |downloaded: u64, total: u64| {
                if total > 0 {
                    *progress.lock().unwrap() = (downloaded as f32 / total as f32) * 100.0;
                }
            };
            
            let result = if verify_mode {
                updater.verify_files(status_cb, log_cb, progress_cb)
            } else {
                updater.do_update(false, status_cb, log_cb, progress_cb)
            };
            
            match result {
                Ok(_) => {
                    if verify_mode {
                        // After verify, show exit button
                        *button_text.lock().unwrap() = "Exit".to_string();
                        *button_enabled.lock().unwrap() = true;
                    } else {
                        // After update, try to launch main app
                        if let Err(e) = updater.launch_main_app() {
                            log_cb(&format!("Failed to launch main app: {}", e));
                        }
                    }
                }
                Err(e) => {
                    status_cb(&format!("Error: {}", e));
                    log_cb(&format!("Update failed: {}", e));
                    *button_enabled.lock().unwrap() = true;
                }
            }
        });
    }
    
    fn start_verify(&mut self) {
        *self.verify_mode.lock().unwrap() = true;
        self.start_update();
    }
}

impl eframe::App for UpdaterApp {
    fn update(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        egui::CentralPanel::default()
            .frame(egui::Frame::none().fill(self.theme_colors.main_bg))
            .show(ctx, |ui| {
                ui.vertical(|ui| {
                    // Header
                    ui.with_layout(egui::Layout::left_to_right(egui::Align::Center), |ui| {
                        ui.add_space(5.0);
                        ui.label(egui::RichText::new("TcNo Account Switcher Updater")
                            .color(egui::Color32::WHITE)
                            .size(14.0));
                        ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                            if ui.button("X").clicked() {
                                std::process::exit(0);
                            }
                            if ui.button("-").clicked() {
                                // Minimize window - requires platform-specific implementation
                                // For now, this is a placeholder
                                // TODO: Implement window minimize using eframe viewport API
                            }
                        });
                    });
                    
                    ui.add_space(5.0);
                    ui.separator();
                    
                    // Log box
                    egui::ScrollArea::vertical()
                        .max_height(200.0)
                        .show(ui, |ui| {
                            let log_lines = self.log_lines.lock().unwrap();
                            for line in log_lines.iter() {
                                ui.label(egui::RichText::new(line).color(egui::Color32::WHITE));
                            }
                        });
                    
                    ui.add_space(10.0);
                    
                    // Status
                    let status = self.status.lock().unwrap().clone();
                    ui.label(egui::RichText::new(&status).color(egui::Color32::WHITE));
                    
                    // Progress bar
                    let progress = *self.progress.lock().unwrap();
                    ui.add(egui::ProgressBar::new(progress / 100.0)
                        .fill(self.theme_colors.highlight)
                        .show_percentage());
                    
                    ui.add_space(10.0);
                    
                    // Button
                    let button_text = self.button_text.lock().unwrap().clone();
                    let button_enabled = *self.button_enabled.lock().unwrap();
                    
                    ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                        let button = egui::Button::new(&button_text)
                            .fill(self.theme_colors.button_bg)
                            .stroke(egui::Stroke::new(1.0, self.theme_colors.button_border));
                        
                        if ui.add_enabled(button_enabled, button).clicked() {
                            if button_text == "Start update" {
                                self.start_update();
                            } else if button_text == "Verify files" {
                                self.start_verify();
                            } else if button_text == "Exit" {
                                std::process::exit(0);
                            }
                        }
                    });
                });
            });
    }
}

