import { Routes, Route } from 'react-router-dom';
import './App.css';
import QuestionWorkbenchPage from './pages/QuestionWorkbenchPage';
import PracticePage from './pages/PracticePage';
import SettingsPage from './pages/SettingsPage';
import StatsPage from './pages/StatsPage';
import ArchivePage from './pages/ArchivePage';

function App() {
  return (
    <Routes>
      <Route path="/" element={<QuestionWorkbenchPage />} />
      <Route path="/settings" element={<SettingsPage />} />
      <Route path="/stats" element={<StatsPage />} />
      <Route path="/archive" element={<ArchivePage />} />
      <Route path="/practice" element={<PracticePage />} />
      <Route path="/practice/:sessionId" element={<PracticePage />} />
    </Routes>
  );
}

export default App;
