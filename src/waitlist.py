"""
API Router for the Waitlist / Lead Capture.
Exposes an endpoint that external sites (like Framer) can hit to register interest.
Uses a simple SQLite database to avoid blocking on Postgres migrations for MVP.
"""

import os
import sqlite3
import logging
from datetime import datetime
from pydantic import BaseModel, EmailStr
from fastapi import APIRouter, HTTPException, status

logger = logging.getLogger(__name__)

# Lightweight local DB for waitlist leads
DB_PATH = os.path.join(os.path.dirname(__file__), "..", "data", "waitlist.db")

router = APIRouter(prefix="/v1", tags=["waitlist"])

class WaitlistRequest(BaseModel):
    email: EmailStr
    source: str = "organic"

def init_db():
    """Initialize the SQLite schema if it doesn't exist."""
    os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute('''
        CREATE TABLE IF NOT EXISTS leads (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            email TEXT UNIQUE NOT NULL,
            source TEXT,
            timestamp TEXT NOT NULL
        )
    ''')
    conn.commit()
    conn.close()

# Initialize DB on module load
init_db()

@router.post("/waitlist", status_code=status.HTTP_201_CREATED)
async def join_waitlist(req: WaitlistRequest):
    """
    Endpoint for external landing pages (Framer, Webflow) to capture leads.
    Expects CORS to be configured in main.py.
    """
    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.cursor()
        
        # Insert lead, ignore if email already exists (fail silently for user UX)
        cursor.execute(
            "INSERT OR IGNORE INTO leads (email, source, timestamp) VALUES (?, ?, ?)",
            (req.email, req.source, datetime.now().isoformat())
        )
        
        # Check if we actually inserted it (rowcount == 1)
        if cursor.rowcount == 1:
            logger.info(f"✨ New Waitlist Signup: {req.email} (Source: {req.source})")
        
        conn.commit()
        conn.close()
        
        return {"status": "success", "message": "Added to waitlist"}
        
    except Exception as e:
        logger.error(f"Waitlist Error: {str(e)}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Failed to process waitlist request"
        )
