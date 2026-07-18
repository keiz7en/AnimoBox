import React, { useEffect } from 'react';
import ReactDOM from 'react-dom/client';
import { MantineProvider } from '@mantine/core';
import '@mantine/core/styles.css';
import App from './App';
import './styles/global.css';

function Root() {
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', 'deep-purple-amber');
  }, []);

  return (
    <MantineProvider
      theme={{
        primaryColor: 'amber',
        primaryShade: { light: 5, dark: 5 },
        colors: {
          amber: ['#fff8e1', '#ffecb3', '#ffe082', '#ffd54f', '#ffca28', '#ffc107', '#ffb300', '#ffa000', '#ff8f00', '#ff6f00'],
          deepPurple: ['#ede7f6', '#d1c4e9', '#b39ddb', '#9575cd', '#7e57c2', '#673ab7', '#5e35b1', '#512da8', '#4527a0', '#311b92'],
          dark: ['#c1c2c5', '#a6a7ab', '#909296', '#5c5f66', '#373a40', '#2c2e33', '#25262b', '#1a1b1e', '#141517', '#101113'],
          gray: ['#f8f9fa', '#e9ecef', '#dee2e6', '#ced4da', '#adb5bd', '#868e96', '#495057', '#343a40', '#212529', '#121218'],
        },
        fontFamily: "'Roboto', 'Helvetica Neue', Arial, sans-serif",
        fontSizes: {
          xs: '11px',
          sm: '13px',
          md: '14px',
          lg: '16px',
          xl: '20px',
        },
        defaultRadius: 'sm',
        radius: {
          xs: '2px',
          sm: '4px',
          md: '4px',
          lg: '8px',
          xl: '8px',
        },
        components: {
          Button: { defaultProps: { radius: 'sm' } },
          Card: { defaultProps: { radius: 'sm' } },
          TextInput: { defaultProps: { radius: 'sm', size: 'sm' } },
          ActionIcon: { defaultProps: { radius: 'sm' } },
          Badge: { defaultProps: { radius: 'sm' } },
          Select: { defaultProps: { radius: 'sm' } },
          Tabs: { defaultProps: { radius: 'sm' } },
          Modal: { defaultProps: { radius: 'lg' } },
          Paper: { defaultProps: { radius: 'sm' } },
          Image: { defaultProps: { radius: 'sm' } },
          Chip: { defaultProps: { radius: 'sm' } },
          Input: { defaultProps: { radius: 'sm' } },
          InputWrapper: { defaultProps: { radius: 'sm' } },
          Tooltip: { defaultProps: { radius: 'sm' } },
          Notification: { defaultProps: { radius: 'sm' } },
          Switch: { defaultProps: { radius: 'sm' } },
          Slider: { defaultProps: { radius: 'xs' } },
        },
      }}
    >
      <App />
    </MantineProvider>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>
);
