import React, { useState, useEffect } from 'react';

interface CountdownTimerProps {
  airingAt: number;
  nextEp?: number;
}

function formatCountdown(diffMs: number): string {
  if (diffMs <= 0) return 'Aired';
  const totalSeconds = Math.floor(diffMs / 1000);
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return `${days}d ${hours}h ${minutes}m ${seconds}s`;
}

function formatFullDate(unixTimestamp: number): string {
  const d = new Date(unixTimestamp * 1000);
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  const hours = d.getHours();
  const minutes = String(d.getMinutes()).padStart(2, '0');
  const ampm = hours >= 12 ? 'PM' : 'AM';
  const h12 = hours % 12 || 12;
  return `${year}/${month}/${day} ${h12}:${minutes} ${ampm} GMT`;
}

export default function CountdownTimer({ airingAt, nextEp }: CountdownTimerProps) {
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    const interval = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  if (!airingAt || airingAt <= 0) return null;

  const diffMs = airingAt * 1000 - now;
  const epLabel = nextEp ? `Episode ${nextEp}` : 'next episode';
  const dateStr = formatFullDate(airingAt);

  if (diffMs <= 0) {
    return (
      <div className="countdown-timer aired">
        {epLabel} has aired!
      </div>
    );
  }

  const countdown = formatCountdown(diffMs);

  return (
    <div className="countdown-timer">
      The {epLabel} is predicted to arrive on {dateStr} ({countdown})
    </div>
  );
}
