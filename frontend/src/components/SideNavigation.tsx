import React from 'react';
import { NavLink } from 'react-router-dom';
import { cx } from '../utils';
import type { NavItem } from '../types';
import { LibraryIcon, BookMarkedIcon, BarChartIcon, ArchiveIcon } from './icons';

const navItems: NavItem[] = [
  { key: 'question-center', label: '题库', icon: <LibraryIcon />, path: '/' },
  { key: 'practice', label: '做题模式', icon: <BookMarkedIcon />, path: '/practice' },
  { key: 'stats', label: '学习统计', icon: <BarChartIcon />, path: '/stats' },
  { key: 'archive', label: '错题归档', icon: <ArchiveIcon />, path: '/archive' },
];

export default function SideNavigation() {

  return (
    <aside className="sidenav">
      <nav aria-label="主导航">
        <div className="sidenav-list">
          {navItems.map((item) => (
            <NavLink
              key={item.key}
              to={item.path ?? '/'}
              className={({ isActive }) =>
                cx('sidenav-item', isActive && 'is-active')
              }
            >
              {item.icon}
              <span className="sidenav-label">{item.label}</span>
            </NavLink>
          ))}
        </div>
      </nav>
    </aside>
  );
}
