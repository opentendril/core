import json
import os
import hmac
import hashlib

from typing import List, Dict

from .config import SECRET_KEY

class SkillsManager:
    def __init__(self):
        self.skills_dir = "/app/skills"
        self.dynamic_dir = "/app/data/dynamic_skills"
        self.skills: List[Dict] = []
        self.load_skills()

    def load_skills(self):
        self.skills = []
        for dir_path in [self.skills_dir, self.dynamic_dir]:
            if os.path.exists(dir_path):
                for filename in os.listdir(dir_path):
                    if filename.endswith(".skill.json"):
                        file_path = os.path.join(dir_path, filename)
                        try:
                            with open(file_path, "r") as f:
                                data = json.load(f)
                            content_str = json.dumps({k: v for k, v in data.items() if k != "signature"}, sort_keys=True)
                            expected_sig = hmac.new(SECRET_KEY.encode(), content_str.encode(), hashlib.sha256).hexdigest()
                            if hmac.compare_digest(data.get("signature", ""), expected_sig):
                                self.skills.append(data)
                        except Exception:
                            pass  # Invalid file/signature

    def get_context(self) -> str:
        contexts = []
        for skill in self.skills:
            ctx = f"{skill['name']}: {skill.get('description', '')}\n{skill.get('system_prompt', '')}"
            contexts.append(ctx)
        return "\n\n".join(contexts)
