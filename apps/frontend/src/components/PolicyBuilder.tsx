import React, { useState, useEffect } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Box,
  Typography,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Chip,
  Switch,
  FormControlLabel,
  Grid,
  Paper,
  Divider,
} from '@mui/material';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import {
  useSortable,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';

import { Policy, PolicyItem, Team, Model, RequestLimits, TimeRange } from '../types';

interface PolicyBuilderProps {
  open: boolean;
  policy?: Policy | null;
  teams: Team[];
  models: Model[];
  onSave: (policy: Policy) => void;
  onClose: () => void;
}

// Draggable item component
const SortableItem = ({ id, children }: { id: string; children: React.ReactNode }) => {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
  } = useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      {children}
    </div>
  );
};

const PolicyBuilder: React.FC<PolicyBuilderProps> = ({
  open,
  policy,
  teams,
  models,
  onSave,
  onClose,
}) => {
  const [formData, setFormData] = useState<Partial<Policy>>({
    name: '',
    description: '',
    items: [],
    requestLimits: {
      tokenLimit: 1000,
      timePeriod: 'hour',
    },
    timeRange: {
      startTime: '09:00',
      endTime: '17:00',
      unlimited: false,
    },
    isActive: true,
  });

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  useEffect(() => {
    if (policy) {
      setFormData(policy);
    } else {
      setFormData({
        name: '',
        description: '',
        items: [],
        requestLimits: {
          tokenLimit: 1000,
          timePeriod: 'hour',
        },
        timeRange: {
          startTime: '09:00',
          endTime: '17:00',
          unlimited: false,
        },
        isActive: true,
      });
    }
  }, [policy, open]);

  const handleInputChange = (field: string, value: any) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleRequestLimitsChange = (field: keyof RequestLimits, value: any) => {
    setFormData(prev => ({
      ...prev,
      requestLimits: {
        ...prev.requestLimits!,
        [field]: value,
      },
    }));
  };

  const handleTimeRangeChange = (field: keyof TimeRange, value: any) => {
    setFormData(prev => ({
      ...prev,
      timeRange: {
        ...prev.timeRange!,
        [field]: value,
      },
    }));
  };

  const addPolicyItem = (type: 'tier' | 'model', value: string, isApprove: boolean) => {
    const newItem: PolicyItem = {
      id: `item-${Date.now()}`,
      type,
      value,
      isApprove,
    };

    setFormData(prev => ({
      ...prev,
      items: [...(prev.items || []), newItem],
    }));
  };

  const removePolicyItem = (itemId: string) => {
    setFormData(prev => ({
      ...prev,
      items: prev.items?.filter(item => item.id !== itemId) || [],
    }));
  };

  const handleDragEnd = (event: any) => {
    const { active, over } = event;

    if (active.id !== over.id) {
      setFormData(prev => {
        const items = prev.items || [];
        const oldIndex = items.findIndex(item => item.id === active.id);
        const newIndex = items.findIndex(item => item.id === over.id);

        return {
          ...prev,
          items: arrayMove(items, oldIndex, newIndex),
        };
      });
    }
  };

  const handleSave = () => {
    if (!formData.name || !formData.description) {
      return;
    }

    const policyToSave: Policy = {
      id: policy?.id || '',
      name: formData.name,
      description: formData.description,
      items: formData.items || [],
      requestLimits: formData.requestLimits!,
      timeRange: formData.timeRange!,
      created: policy?.created || new Date().toISOString(),
      modified: new Date().toISOString(),
      type: policy?.type || 'auth', // Default to auth type
      namespace: policy?.namespace || 'llm', // Default to llm namespace
      isActive: formData.isActive,
    };

    onSave(policyToSave);
  };

  const getModelName = (modelId: string) => models.find(m => m.id === modelId)?.name || modelId;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        {policy ? 'Edit Policy' : 'Create New Policy'}
      </DialogTitle>
      
      <DialogContent>
        <Box sx={{ mt: 2 }}>
          {/* Basic Information */}
          <Typography variant="h6" gutterBottom>
            Basic Information
          </Typography>
          
          <Grid container spacing={2} sx={{ mb: 3 }}>
            <Grid item xs={12}>
              <TextField
                fullWidth
                label="Policy Name"
                value={formData.name}
                onChange={(e) => handleInputChange('name', e.target.value)}
                required
              />
            </Grid>
            <Grid item xs={12}>
              <TextField
                fullWidth
                label="Description"
                value={formData.description}
                onChange={(e) => handleInputChange('description', e.target.value)}
                multiline
                rows={2}
                required
              />
            </Grid>
            <Grid item xs={12}>
              <FormControlLabel
                control={
                  <Switch
                    checked={formData.isActive}
                    onChange={(e) => handleInputChange('isActive', e.target.checked)}
                  />
                }
                label="Active"
              />
            </Grid>
          </Grid>

          <Divider sx={{ my: 3 }} />

          {/* Policy Rules */}
          <Typography variant="h6" gutterBottom>
            Policy Rules
          </Typography>
          
          <Box sx={{ mb: 2 }}>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              Add tiers and models to this policy. Green items are approved, red items are denied.
            </Typography>
          </Box>

          {/* Add Rule Buttons */}
          <Grid container spacing={1} sx={{ mb: 2 }}>
            <Grid item>
              <FormControl size="small" sx={{ minWidth: 120 }}>
                <InputLabel>Add Tier</InputLabel>
                <Select
                  label="Add Tier"
                  value=""
                  onChange={(e) => {
                    if (e.target.value) {
                      addPolicyItem('tier', e.target.value, true);
                    }
                  }}
                >
                  {['free', 'premium', 'enterprise'].map(tier => (
                    <MenuItem key={tier} value={tier}>
                      {tier.charAt(0).toUpperCase() + tier.slice(1)}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid item>
              <FormControl size="small" sx={{ minWidth: 120 }}>
                <InputLabel>Add Model</InputLabel>
                <Select
                  label="Add Model"
                  value=""
                  onChange={(e) => {
                    if (e.target.value) {
                      addPolicyItem('model', e.target.value, true);
                    }
                  }}
                >
                  {models.map(model => (
                    <MenuItem key={model.id} value={model.id}>
                      {model.name}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          </Grid>

          {/* Policy Items */}
          {formData.items && formData.items.length > 0 && (
            <Paper sx={{ p: 2, mb: 3 }}>
              <DndContext
                sensors={sensors}
                collisionDetection={closestCenter}
                onDragEnd={handleDragEnd}
              >
                <SortableContext
                  items={formData.items.map(item => item.id)}
                  strategy={verticalListSortingStrategy}
                >
                  {formData.items.map((item) => (
                    <SortableItem key={item.id} id={item.id}>
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          p: 1,
                          mb: 1,
                          bgcolor: 'background.default',
                          borderRadius: 1,
                          cursor: 'grab',
                        }}
                      >
                        <Chip
                          label={`${item.type === 'tier' ? item.value.charAt(0).toUpperCase() + item.value.slice(1) : getModelName(item.value)}`}
                          color={item.isApprove ? 'success' : 'error'}
                          variant={item.isApprove ? 'filled' : 'outlined'}
                        />
                        <Box>
                          <Button
                            size="small"
                            onClick={() => {
                              const updatedItems = formData.items?.map(i =>
                                i.id === item.id ? { ...i, isApprove: !i.isApprove } : i
                              );
                              setFormData(prev => ({ ...prev, items: updatedItems }));
                            }}
                          >
                            Toggle
                          </Button>
                          <Button
                            size="small"
                            color="error"
                            onClick={() => removePolicyItem(item.id)}
                          >
                            Remove
                          </Button>
                        </Box>
                      </Box>
                    </SortableItem>
                  ))}
                </SortableContext>
              </DndContext>
            </Paper>
          )}

          <Divider sx={{ my: 3 }} />

          {/* Request Limits */}
          <Typography variant="h6" gutterBottom>
            Request Limits
          </Typography>
          
          <Grid container spacing={2} sx={{ mb: 3 }}>
            <Grid item xs={6}>
              <TextField
                fullWidth
                label="Token Limit"
                type="number"
                value={formData.requestLimits?.tokenLimit || ''}
                onChange={(e) => handleRequestLimitsChange('tokenLimit', 
                  e.target.value ? parseInt(e.target.value) : null
                )}
                helperText="Leave empty for unlimited"
              />
            </Grid>
            <Grid item xs={6}>
              <FormControl fullWidth>
                <InputLabel>Time Period</InputLabel>
                <Select
                  value={formData.requestLimits?.timePeriod || 'hour'}
                  onChange={(e) => handleRequestLimitsChange('timePeriod', e.target.value)}
                  label="Time Period"
                >
                  <MenuItem value="hour">Per Hour</MenuItem>
                  <MenuItem value="day">Per Day</MenuItem>
                  <MenuItem value="week">Per Week</MenuItem>
                  <MenuItem value="month">Per Month</MenuItem>
                </Select>
              </FormControl>
            </Grid>
          </Grid>

          <Divider sx={{ my: 3 }} />

          {/* Time Range */}
          <Typography variant="h6" gutterBottom>
            Time Range
          </Typography>
          
          <FormControlLabel
            control={
              <Switch
                checked={formData.timeRange?.unlimited}
                onChange={(e) => handleTimeRangeChange('unlimited', e.target.checked)}
              />
            }
            label="24/7 Access"
            sx={{ mb: 2 }}
          />

          {!formData.timeRange?.unlimited && (
            <Grid container spacing={2}>
              <Grid item xs={6}>
                <TextField
                  fullWidth
                  label="Start Time"
                  type="time"
                  value={formData.timeRange?.startTime || '09:00'}
                  onChange={(e) => handleTimeRangeChange('startTime', e.target.value)}
                  InputLabelProps={{ shrink: true }}
                />
              </Grid>
              <Grid item xs={6}>
                <TextField
                  fullWidth
                  label="End Time"
                  type="time"
                  value={formData.timeRange?.endTime || '17:00'}
                  onChange={(e) => handleTimeRangeChange('endTime', e.target.value)}
                  InputLabelProps={{ shrink: true }}
                />
              </Grid>
            </Grid>
          )}
        </Box>
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          onClick={handleSave}
          variant="contained"
          disabled={!formData.name || !formData.description}
        >
          {policy ? 'Update' : 'Create'} Policy
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default PolicyBuilder;