import { render, screen } from '@testing-library/react'
import { FieldMapper } from './FieldMapper.jsx'
import { useMatcherStore } from '../store/useMatcherStore.js'

beforeEach(() => {
  // Mock fetch to avoid actual API calls in tests
  global.fetch = vi.fn()
  useMatcherStore.setState({ 
    batchID: 'benchmark-batch-001', 
    error: null, 
    loading: false,
    config: {
      column_mapping: undefined
    }
  })
})

describe('FieldMapper', () => {
  it('TEST: date selects offer an explicit none option', async () => {
    render(<FieldMapper 
      availableSourceCols={['CustID','CustomerName','TxDate']} 
      availableDestCols={['client_ref','client_name','tx_date']} 
    />)

    const label = screen.getByText('Source Date Column')
    const select = label.closest('div').querySelector('select')

    const noneOption = select.querySelector('option[value=""]')
    expect(noneOption).toBeInTheDocument()
  })

  it('TEST: an unset date column does not display a real column', async () => {
    useMatcherStore.setState({ 
      config: {
        column_mapping: {
          name_fields_src: ['CustomerName'],
          name_fields_dest: ['client_name'],
          ref_id_src: 'CustID',
          ref_id_dest: 'client_ref',
          date_field_src: '',
          date_field_dest: '',
          secondary_fields: [],
        }
      }
    })

    render(<FieldMapper 
      availableSourceCols={['CustID','CustomerName','TxDate']} 
      availableDestCols={['client_ref','client_name','tx_date']} 
    />)

    const label = screen.getByText('Source Date Column')
    const select = label.closest('div').querySelector('select')

    expect(select.value).toBe('')
    expect(select.value).not.toBe('CustID')
  })

  it('TEST: a saved column missing from the list stays visible', async () => {
    useMatcherStore.setState({ 
      config: {
        column_mapping: {
          ref_id_src: 'OLD_COL',
          ref_id_dest: 'client_ref',
          name_fields_src: ['CustomerName'],
          name_fields_dest: ['client_name'],
          date_field_src: '',
          date_field_dest: '',
          secondary_fields: [],
        }
      }
    })

    render(<FieldMapper 
      availableSourceCols={['CustID','CustomerName']} 
      availableDestCols={['client_ref','client_name']} 
    />)

    const label = screen.getByText('Source Reference ID Column')
    const select = label.closest('div').querySelector('select')

    // Verify the missing column is present in the options
    expect(select).toContainHTML('OLD_COL')
    expect(select.value).toBe('OLD_COL')
    expect(select.value).not.toBe('CustID')
  })

  it('TEST: reference id placeholder differs from date placeholder', async () => {
    render(<FieldMapper 
      availableSourceCols={['CustID','CustomerName']} 
      availableDestCols={['client_ref','client_name']} 
    />)

    const refLabel = screen.getByText('Source Reference ID Column')
    const dateLabel = screen.getByText('Source Date Column')

    const refSelect = refLabel.closest('div').querySelector('select')
    const dateSelect = dateLabel.closest('div').querySelector('select')

    const refPlaceholder = refSelect.querySelector('option[value=""]').textContent
    const datePlaceholder = dateSelect.querySelector('option[value=""]').textContent

    expect(refPlaceholder).not.toBe(datePlaceholder)
  })
})
