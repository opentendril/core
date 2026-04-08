#!/usr/bin/env python3
import os
import sys

# Append parent dir so we can import src
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from src.gitmanager import GitManager

def submit_test_pr():
    print("Initiating Tendril GitManager PR Submission test...")
    g = GitManager()
    
    # 1. Create a new branch
    branch_name = "tendril-marketing-update"
    try:
        print(g.create_branch(branch_name))
    except Exception as e:
        print(f"Branch already exists or error: {e}")
        g.checkout(branch_name)
        
    # 2. Make a fast edit to PROGRESS.md
    with open("PROGRESS.md", "a") as f:
        f.write("\n- Deployed zero-touch Marketing Engine natively via autonomous PR flow.\n")
        
    # 3. Commit
    print(g.commit_changes("feat(marketing): deploy automated social asset engine"))
    
    # 4. Push Branch
    try:
        # Tendril might not have push rights locally but it will try
        print(g.push_branch(branch_name))
    except Exception as e:
        print(f"Warning on push: {e}")
    
    # 5. Create Pull Request
    print("Submitting PR via PyGithub...")
    result = g.create_pull_request(
        repo_name="opentendril/core", 
        title="feat: deploy marketing engine", 
        body="This Pull Request was autonomously submitted by Tendril via the GitManager to deploy the Zero-Touch Marketing Engine.", 
        head_branch=branch_name
    )
    print(result)

if __name__ == "__main__":
    submit_test_pr()
