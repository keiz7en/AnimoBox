import React from 'react';
import { NavLink } from 'react-router-dom';
import {
  IconHome,
  IconSearch,
  IconLibrary,
  IconSettings,
  IconPlayerPlay,
  IconHistory,
} from '@tabler/icons-react';

const navItems = [
  { to: '/', icon: IconHome, label: 'Home' },
  { to: '/search', icon: IconSearch, label: 'Search' },
  { to: '/library', icon: IconLibrary, label: 'Library' },
  { to: '/history', icon: IconHistory, label: 'History' },
  { to: '/settings', icon: IconSettings, label: 'Settings' },
];

export default function Navbar() {
  return (
    <nav
      style={{
        width: 64,
        height: '100vh',
        background: 'var(--bg-nav)',
        borderRight: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        paddingTop: 16,
        gap: 4,
        position: 'fixed',
        left: 0,
        top: 0,
        zIndex: 100,
      }}
    >
      <div
        style={{
          width: 40,
          height: 40,
          background: 'var(--accent)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          marginBottom: 24,
          borderRadius: 'var(--radius)',
        }}
      >
        <IconPlayerPlay size={20} color="#000" />
      </div>

      {navItems.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          style={{ textDecoration: 'none', width: '100%', display: 'flex', justifyContent: 'center' }}
        >
          {({ isActive }) => (
            <div
              data-tooltip={item.label}
              style={{
                width: 48,
                height: 48,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: isActive ? 'var(--accent)' : 'transparent',
                color: isActive ? '#000' : 'var(--text-sub)',
                borderRadius: 'var(--radius)',
                cursor: 'pointer',
                transition: 'all var(--t)',
                borderLeft: isActive ? '3px solid var(--accent)' : '3px solid transparent',
              }}
            >
              <item.icon size={20} />
            </div>
          )}
        </NavLink>
      ))}
    </nav>
  );
}
