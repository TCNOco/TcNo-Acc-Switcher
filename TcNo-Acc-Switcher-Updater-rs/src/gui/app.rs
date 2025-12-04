// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2025 TroubleChute (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

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
    updates_available: usize,
    theme_colors: ThemeColors,
}

#[derive(Clone)]
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
        };
        
        // Load theme
        app.load_theme();
        
        // Initialize updater
        thread::spawn({
            let status = app.status.clone();
            let log_lines = app.log_lines.clone();
            let button_text = app.button_text.clone();
            let button_enabled = app.button_enabled.clone();
            
            move || {
                match Updater::new() {
                    Ok(mut updater) => {
                        *status.lock().unwrap() = "Checking for updates".to_string();
                        Logger::write_line("Checking for updates");
                        
                        match updater.get_updates_list() {
                            Ok(updates) => {
                                let count = updates.len();
                                if count > 0 {
                                    *button_text.lock().unwrap() = "Start update".to_string();
                                    *button_enabled.lock().unwrap() = true;
                                    *status.lock().unwrap() = "Waiting for user input...".to_string();
                                    
                                    for (version, changes) in &updates {
                                        let log = format!("Update found: {}\n- {}", version, changes);
                                        log_lines.lock().unwrap().push(log);
                                        Logger::write_line(&format!("Update found: {}", version));
                                    }
                                    
                                    log_lines.lock().unwrap().push(format!("Total updates found: {}", count));
                                } else {
                                    *button_text.lock().unwrap() = "Exit".to_string();
                                    *button_enabled.lock().unwrap() = true;
                                    *status.lock().unwrap() = ":)".to_string();
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
        if let Ok(user_data) = crate::utils::get_user_data_folder(&crate::utils::get_app_data_folder().unwrap_or_default()) {
            let window_settings = user_data.join("WindowSettings.json");
            if window_settings.exists() {
                    if let Ok(content) = crate::utils::read_all_text(&window_settings) {
                        if let Ok(json) = serde_json::from_str::<serde_json::Value>(&content) {
                            if let Some(theme_name) = json.get("ActiveTheme").and_then(|v| v.as_str()) {
                                let theme_file = user_data.join("themes").join(theme_name).join("info.yaml");
                                if theme_file.exists() {
                                    if let Ok(yaml_content) = crate::utils::read_all_text(&theme_file) {
                                        if let Ok(_theme_data) = serde_yaml::from_str::<serde_yaml::Value>(&yaml_content) {
                                            // Parse theme colors (simplified)
                                            self.theme_colors = ThemeColors::default(); // Use defaults for now
                                        }
                                    }
                                }
                            }
                        }
                    }
            }
        }
    }
    
    fn write_line(&self, line: &str) {
        self.log_lines.lock().unwrap().push(line.to_string());
        Logger::write_line(line);
    }
    
    fn set_status(&self, status: &str) {
        *self.status.lock().unwrap() = status.to_string();
        Logger::write_line(&format!("Status: {}", status));
    }
    
    fn start_update(&mut self) {
        *self.button_text.lock().unwrap() = "Started".to_string();
        *self.button_enabled.lock().unwrap() = false;
        
        let status = self.status.clone();
        let log_lines = self.log_lines.clone();
        let progress = self.progress.clone();
        
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
            
            if let Err(e) = updater.do_update(false, status_cb, log_cb, progress_cb) {
                status_cb(&format!("Error: {}", e));
                log_cb(&format!("Update failed: {}", e));
            }
        });
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
                                // Minimize (would need platform-specific code)
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
                            if button_text == "Start update" || button_text == "Verify files" {
                                self.start_update();
                            } else if button_text == "Exit" {
                                std::process::exit(0);
                            }
                        }
                    });
                });
            });
    }
}

