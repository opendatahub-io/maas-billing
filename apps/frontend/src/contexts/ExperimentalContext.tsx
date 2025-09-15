import React, { createContext, useContext } from 'react';

interface ExperimentalContextType {
  experimentalMode: boolean;
}

const ExperimentalContext = createContext<ExperimentalContextType>({
  experimentalMode: false,
});

export const useExperimental = () => useContext(ExperimentalContext);

export const ExperimentalProvider: React.FC<{ experimentalMode: boolean; children: React.ReactNode }> = ({ 
  experimentalMode, 
  children 
}) => {
  return (
    <ExperimentalContext.Provider value={{ experimentalMode }}>
      {children}
    </ExperimentalContext.Provider>
  );
};