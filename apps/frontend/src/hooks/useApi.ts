import { useState, useEffect, useCallback } from 'react';
import apiService from '../services/api';

export const useModels = () => {
  const [models, setModels] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchModels = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await apiService.getModels();
      setModels(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch models');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchModels();
  }, [fetchModels]);

  return { models, loading, error, refetch: fetchModels };
};

export const useLiveRequests = (autoRefresh: boolean = true, refreshInterval: number = 10000) => {
  const [requests, setRequests] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  const fetchLiveRequests = useCallback(async (isRefresh: boolean = false) => {
    try {
      if (isRefresh) {
        setRefreshing(true);
      }
      setError(null);
      const data = await apiService.getLiveRequests();
      setRequests(prev => {
        // Create a Map to track unique requests by ID
        const existingIds = new Set(prev.map((req: any) => req.id));
        
        // Only add new requests that don't already exist
        const newRequests = data.filter((req: any) => !existingIds.has(req.id));
        
        // Prepend new requests and keep last 100
        return [...newRequests, ...prev].slice(0, 100);
      });
      setLastUpdated(new Date());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch live requests');
    } finally {
      setLoading(false);
      if (isRefresh) {
        setRefreshing(false);
      }
    }
  }, []);

  useEffect(() => {
    fetchLiveRequests();
  }, [fetchLiveRequests]);

  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(() => {
      fetchLiveRequests(true);
    }, refreshInterval);

    return () => clearInterval(interval);
  }, [autoRefresh, refreshInterval, fetchLiveRequests]);

  return { 
    requests, 
    loading, 
    refreshing, 
    error, 
    lastUpdated, 
    refetch: () => fetchLiveRequests(true) 
  };
};

export const useDashboardStats = (autoRefresh: boolean = true, refreshInterval: number = 15000) => {
  const [stats, setStats] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  const fetchStats = useCallback(async (isRefresh: boolean = false) => {
    try {
      if (isRefresh) {
        setRefreshing(true);
      } else {
        setLoading(true);
      }
      setError(null);
      const data = await apiService.getDashboardStats();
      setStats(data);
      setLastUpdated(new Date());
    } catch (err) {
      console.error('Dashboard stats error:', err);
      setError(err instanceof Error ? err.message : 'Failed to fetch dashboard stats');
    } finally {
      setLoading(false);
      if (isRefresh) {
        setRefreshing(false);
      }
    }
  }, []);

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(() => {
      fetchStats(true);
    }, refreshInterval);

    return () => clearInterval(interval);
  }, [autoRefresh, refreshInterval, fetchStats]);

  return { 
    stats, 
    loading, 
    refreshing, 
    error, 
    lastUpdated,
    refetch: () => fetchStats(true) 
  };
};