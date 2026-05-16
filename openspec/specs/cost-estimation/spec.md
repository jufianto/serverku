## ADDED Requirements

### Requirement: Cost Estimation Display
The CLI SHALL display estimated infrastructure costs.

#### Scenario: Display on List
- **WHEN** the user runs `serverku list`
- **THEN** the output table SHALL include an "Est. Cost" column showing the approximate hourly cost of running VMs and monthly cost of persistent disks.

#### Scenario: Display on Up
- **WHEN** the user successfully runs `serverku up`
- **THEN** the success summary SHALL include the estimated hourly rate for the VM and monthly rate for the attached storage.
