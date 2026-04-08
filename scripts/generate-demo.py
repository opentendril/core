#!/usr/bin/env python3
"""
Marketing Demo Automation
Runs a headless Chromium browser, connects to the Tendril local instance,
and records an MP4 snippet of the self-healing Moat Loop for X/LinkedIn posts.
"""

import os
import time
from playwright.sync_api import sync_playwright

def run_demo():
    # Ensure the output directory exists
    os.makedirs("marketing_assets", exist_ok=True)
    
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        # Record video to marketing_assets folder
        context = browser.new_context(
            record_video_dir="marketing_assets/",
            record_video_size={"width": 1280, "height": 720}
        )
        page = context.new_page()

        print("🎬 Connecting to Tendril Kernel...")
        try:
            page.goto("http://localhost:8080/chat")
        except Exception as e:
            print(f"❌ Could not connect to Tendril. Is the Docker container running? Error: {e}")
            browser.close()
            return
            
        # Wait for the Chat interface to load
        page.wait_for_selector("#chat-input")
        print("✅ Interface loaded. Initiating prompt...")
        
        # We simulate typing out a PR request that Triggers the Moat
        prompt_text = "I need to fix the alignment in main.py, it looks bad. Create an edit to fix it to center align."
        page.fill("#chat-input", prompt_text)
        
        # Pause for cinematic effect
        page.wait_for_timeout(1000)
        
        print("▶️ Sending instruction...")
        page.press("#chat-input", "Enter")
        
        # Wait for the AI's first response bubble to appear
        page.wait_for_selector(".msg-bubble.assistant", state="attached", timeout=10000)
        
        print("⏳ Recording Tendril's autonomous execution (15s)...")
        # Let the agent's SSE response stream for a while to capture the "Thinking" and generation loop
        page.wait_for_timeout(15000)
        
        print("✅ Demo capture complete.")
        
        # Close context first to ensure video is saved
        context.close()
        browser.close()
        print(f"🎥 Video saved to 'marketing_assets/' directory.")

if __name__ == "__main__":
    print("Initializing Playwright Demo Generator...")
    run_demo()
