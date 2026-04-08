#!/usr/bin/env python3
import json
import os
import hmac
import hashlib

# Load SECRET_KEY manually from .env
secret_key_val = "generate_with_openssl_rand_-hex_32"
if os.path.exists(".env"):
    with open(".env", "r") as f:
        for line in f:
            if line.startswith("SECRET_KEY="):
                secret_key_val = line.strip().split("=", 1)[1]
                break

SECRET_KEY = secret_key_val

marketing_skill = {
    "name": "marketing_agent",
    "description": "Zero-Touch Automated Marketing. Drafts X and LinkedIn posts based on recent codebase changes and requests human approval.",
    "system_prompt": "You are the Root Agent Marketing Engine. Your objective is to build awareness for Tendril. "
                     "1. Use the 'run_command' tool to run 'git log -n 5' or read 'PROGRESS.md' to see what was built or changed recently. "
                     "2. Draft engaging social media posts announcing the feature. Use hashtags like #BuildInPublic, #AI, #Tendril. "
                     "3. CRITICAL: You MUST use the 'approval' tool to present your drafts to the user for publishing approval. "
                     "Never assert that you have posted something without explicit approval. "
                     "Stay professional, concise, and futuristic.",
    "is_active": True
}

# The signature process defined by Tendril's SkillsManager
content_str = json.dumps({k: v for k, v in marketing_skill.items() if k != "signature"}, sort_keys=True)
signature = hmac.new(SECRET_KEY.encode(), content_str.encode(), hashlib.sha256).hexdigest()

marketing_skill["signature"] = signature

# Ensure skills dir exists
os.makedirs("skills", exist_ok=True)

with open("skills/marketing-agent.skill.json", "w") as f:
    json.dump(marketing_skill, f, indent=4)

print(f"✅ Successfully signed and wrote skills/marketing-agent.skill.json")
